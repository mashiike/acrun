package acrun

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types"
)

// DeployOption holds the flags of the deploy command.
//
// When KeepVersions is greater than zero, versions older than the newest
// KeepVersions are deleted after the endpoint has been updated. Versions that
// any endpoint references through its live or target version are always kept,
// so a version an endpoint was rolled back to is never deleted.
type DeployOption struct {
	DryRun          bool          `name:"dry-run" help:"dry run" default:"false"`
	EndpointName    *string       `name:"endpoint-name" help:"the endpoint name to deploy. if not specified, use the default endpoint."`
	KeepVersions    int           `name:"keep-versions" help:"keep the specified number of latest versions and delete older ones. versions referenced by any endpoint are always kept. 0 keeps all versions" default:"0"`
	WaitDuration    time.Duration `name:"wait-duration" help:"maximum duration to wait until the agent runtime is ready" default:"30m"`
	PollingInterval time.Duration `name:"polling-interval" help:"polling interval to check the agent runtime status" default:"5s"`
}

// Deploy creates or updates the agent runtime described by the configuration
// file, then points the endpoint named by opt.EndpointName at the resulting
// version, creating that endpoint if it does not exist yet.
//
// Deploying to the DEFAULT endpoint is rejected. Old versions are pruned
// afterwards as described on DeployOption; the prune is skipped when the
// runtime was just created. Pruning is best-effort: failures there are logged
// as warnings and never fail the deploy.
func (app *App) Deploy(ctx context.Context, opt *DeployOption) error {
	e := fillEndpointName(opt.EndpointName)
	if e == DefaultEndpointName {
		return errors.New("deploying to the DEFAULT endpoint is not allowed")
	}
	opt.EndpointName = &e
	if opt.DryRun {
		slog.WarnContext(ctx, "starting deploy in DRY RUN mode. No changes will be made.")
		defer slog.WarnContext(ctx, "ended deploy in DRY RUN mode. No changes were made.")
	}
	agentRuntime, err := app.loadAgentRuntimeFile(ctx)
	if err != nil {
		return fmt.Errorf("load agent runtime file: %w", err)
	}
	var version string
	var created bool
	id, err := app.GetAgentRuntimeIDByName(ctx, *agentRuntime.AgentRuntimeName)
	if err != nil {
		if !errors.Is(err, ErrAgentRuntimeNotFound) {
			return fmt.Errorf("get agent runtime ID by name: %w", err)
		}
		id, version, err = app.createRuntimeAgent(ctx, agentRuntime, opt)
		if err != nil {
			return fmt.Errorf("createRuntimeAgent: %w", err)
		}
		created = true
	} else {
		version, err = app.updateRuntimeAgent(ctx, agentRuntime, opt)
		if err != nil {
			return fmt.Errorf("updateRuntimeAgent: %w", err)
		}
	}
	if !opt.DryRun {
		waiter := &Waiter{
			MaxDuration:   opt.WaitDuration,
			CheckInterval: opt.PollingInterval,
			LogMessage:    "waiting for agent runtime to be ready",
			LogAttributes: []any{"id", id, "version", version},
			Checker: func(ctx context.Context) ([]any, bool, error) {
				out, err := app.ctrlClient.GetAgentRuntime(ctx, &bedrockagentcorecontrol.GetAgentRuntimeInput{
					AgentRuntimeId:      aws.String(id),
					AgentRuntimeVersion: aws.String(version),
				})
				if err != nil {
					return nil, false, fmt.Errorf("GetAgentRuntime: %w", err)
				}
				if out.Status == types.AgentRuntimeStatusReady {
					return []any{"status", out.Status}, true, nil
				}
				return []any{"status", out.Status}, false, nil
			},
		}
		if err := waiter.Wait(ctx); err != nil {
			return fmt.Errorf("waiter.Wait: %w", err)
		}
	}
	slog.InfoContext(ctx, "deployed agent runtime", "name", aws.ToString(agentRuntime.AgentRuntimeName), "id", id, "version", version)
	if err := app.createOrUpdateAgentRuntimeEndpoint(ctx, id, *opt.EndpointName, version, opt); err != nil {
		return fmt.Errorf("createOrUpdateAgentRuntimeEndpoint: %w", err)
	}
	if opt.KeepVersions > 0 && !created {
		app.deleteOldVersions(ctx, id, opt)
	}
	return nil
}

func (app *App) deleteOldVersions(ctx context.Context, id string, opt *DeployOption) {
	inUse, err := app.inUseVersions(ctx, id)
	if err != nil {
		slog.WarnContext(ctx, "skipping deletion of old agent runtime versions: cannot determine which versions endpoints reference", "id", id, "error", err)
		return
	}
	versions, err := app.listAgentRuntimeVersions(ctx, id)
	if err != nil {
		slog.WarnContext(ctx, "skipping deletion of old agent runtime versions: cannot list versions", "id", id, "error", err)
		return
	}
	slices.SortFunc(versions, func(a, b agentRuntimeVersion) int {
		return cmp.Compare(b.number, a.number)
	})
	keepFromListed := opt.KeepVersions
	if opt.DryRun {
		keepFromListed = opt.KeepVersions - 1
	}

	for i, v := range versions {
		if i < keepFromListed {
			continue
		}
		if _, ok := inUse[v.raw]; ok {
			slog.InfoContext(ctx, "keeping agent runtime version referenced by an endpoint", "id", id, "version", v.raw)
			continue
		}
		input := &bedrockagentcorecontrol.DeleteAgentRuntimeInput{
			AgentRuntimeId:      aws.String(id),
			AgentRuntimeVersion: aws.String(v.raw),
		}
		app.DumpIfVerbose(ctx, "DeleteAgentRuntimeInput", input)
		if opt.DryRun {
			slog.InfoContext(ctx, "dry run: delete agent runtime version skipped", "id", id, "version", v.raw)
			continue
		}
		slog.InfoContext(ctx, "deleting agent runtime version", "id", id, "version", v.raw)
		if _, err := app.ctrlClient.DeleteAgentRuntime(ctx, input); err != nil {
			var nfe *types.ResourceNotFoundException
			if errors.As(err, &nfe) {
				slog.InfoContext(ctx, "agent runtime version already deleted", "id", id, "version", v.raw)
				continue
			}
			slog.WarnContext(ctx, "failed to delete agent runtime version", "id", id, "version", v.raw, "error", err)
			continue
		}
		slog.InfoContext(ctx, "deleted agent runtime version", "id", id, "version", v.raw)
	}
}

type agentRuntimeVersion struct {
	number uint64
	raw    string
}

func (app *App) listAgentRuntimeVersions(ctx context.Context, id string) ([]agentRuntimeVersion, error) {
	var versions []agentRuntimeVersion
	p := bedrockagentcorecontrol.NewListAgentRuntimeVersionsPaginator(
		app.ctrlClient,
		&bedrockagentcorecontrol.ListAgentRuntimeVersionsInput{
			AgentRuntimeId: aws.String(id),
		},
	)
	for p.HasMorePages() {
		out, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("ListAgentRuntimeVersions: %w", err)
		}
		for _, rt := range out.AgentRuntimes {
			if rt.Status == types.AgentRuntimeStatusDeleting {
				continue
			}
			raw := aws.ToString(rt.AgentRuntimeVersion)
			number, err := strconv.ParseUint(raw, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("parse agent runtime version %q: %w", raw, err)
			}
			versions = append(versions, agentRuntimeVersion{number: number, raw: raw})
		}
	}
	return versions, nil
}

func (app *App) inUseVersions(ctx context.Context, id string) (map[string]struct{}, error) {
	versions := make(map[string]struct{})
	p := bedrockagentcorecontrol.NewListAgentRuntimeEndpointsPaginator(
		app.ctrlClient,
		&bedrockagentcorecontrol.ListAgentRuntimeEndpointsInput{
			AgentRuntimeId: aws.String(id),
		},
	)
	for p.HasMorePages() {
		out, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("ListAgentRuntimeEndpoints: %w", err)
		}
		for _, e := range out.RuntimeEndpoints {
			name := aws.ToString(e.Name)
			detail, err := app.ctrlClient.GetAgentRuntimeEndpoint(ctx, &bedrockagentcorecontrol.GetAgentRuntimeEndpointInput{
				AgentRuntimeId: aws.String(id),
				EndpointName:   aws.String(name),
			})
			if err != nil {
				return nil, fmt.Errorf("GetAgentRuntimeEndpoint(%s): %w", name, err)
			}
			if v := aws.ToString(detail.LiveVersion); v != "" {
				versions[v] = struct{}{}
			}
			if v := aws.ToString(detail.TargetVersion); v != "" {
				versions[v] = struct{}{}
			}
		}
	}
	return versions, nil
}

func (app *App) createRuntimeAgent(ctx context.Context, agentRuntime *AgentRuntime, opt *DeployOption) (string, string, error) {
	slog.InfoContext(ctx, "creating agent runtime", "name", aws.ToString(agentRuntime.AgentRuntimeName))
	app.DumpIfVerbose(ctx, "CreateAgentRuntimeInput", agentRuntime)
	if opt.DryRun {
		slog.DebugContext(ctx, "dry run: create agent runtime skipped")
		return "(known after deploy)", "(known after deploy)", nil
	}
	resp, err := app.ctrlClient.CreateAgentRuntime(ctx, agentRuntime)
	if err != nil {
		return "", "", fmt.Errorf("CreateAgentRuntime: %w", err)
	}
	var workloadIdentityARN *string
	if resp.WorkloadIdentityDetails != nil {
		workloadIdentityARN = resp.WorkloadIdentityDetails.WorkloadIdentityArn
	}

	slog.DebugContext(ctx, "created agent runtime",
		"arn", aws.ToString(resp.AgentRuntimeArn),
		"version", aws.ToString(resp.AgentRuntimeVersion),
		"id", aws.ToString(resp.AgentRuntimeId),
		"workloadIdentityARN", aws.ToString(workloadIdentityARN),
	)
	return aws.ToString(resp.AgentRuntimeId), aws.ToString(resp.AgentRuntimeVersion), nil
}

func (app *App) updateRuntimeAgent(ctx context.Context, agentRuntime *AgentRuntime, opt *DeployOption) (string, error) {
	out, err := app.GetAgentRuntime(ctx, agentRuntime.AgentRuntimeName, opt.EndpointName)
	if err != nil {
		if !errors.Is(err, ErrAgentRuntimeNotFound) {
			return "", fmt.Errorf("get remote agent(endpoint=%s) : %w", aws.ToString(opt.EndpointName), err)
		}
		// fallback to DEFAULT
		out, err = app.GetAgentRuntime(ctx, agentRuntime.AgentRuntimeName, aws.String(DefaultEndpointName))
		if err != nil {
			return "", fmt.Errorf("get remote agent runtime(endpoint=%s): %w", DefaultEndpointName, err)
		}
	}
	slog.InfoContext(ctx, "updating agent runtime", "name", aws.ToString(agentRuntime.AgentRuntimeName), "arn", aws.ToString(out.AgentRuntimeArn))
	input, err := newUpdateAgentRuntimeInput(out, agentRuntime)
	if err != nil {
		return "", fmt.Errorf("newUpdateAgentRuntimeInput: %w", err)
	}
	app.DumpIfVerbose(ctx, "UpdateAgentRuntimeInput", input)
	if opt.DryRun {
		slog.DebugContext(ctx, "dry run: update agent runtime skipped")
		return "(known after deploy)", nil
	}
	resp, err := app.ctrlClient.UpdateAgentRuntime(ctx, input)
	if err != nil {
		return "", fmt.Errorf("UpdateAgentRuntime: %w", err)
	}
	var workloadIdentityARN *string
	if resp.WorkloadIdentityDetails != nil {
		workloadIdentityARN = resp.WorkloadIdentityDetails.WorkloadIdentityArn
	}
	slog.DebugContext(ctx, "updated agent runtime",
		"arn", aws.ToString(resp.AgentRuntimeArn),
		"version", aws.ToString(resp.AgentRuntimeVersion),
		"id", aws.ToString(resp.AgentRuntimeId),
		"workloadIdentityARN", aws.ToString(workloadIdentityARN),
	)
	return aws.ToString(resp.AgentRuntimeVersion), nil
}

func coalesce[T any](args ...*T) *T {
	for _, arg := range args {
		if arg != nil {
			return arg
		}
	}
	return nil
}

func (app *App) createOrUpdateAgentRuntimeEndpoint(ctx context.Context, id string, endpointName string, version string, opt *DeployOption) error {
	if current, err := app.ctrlClient.GetAgentRuntimeEndpoint(ctx, &bedrockagentcorecontrol.GetAgentRuntimeEndpointInput{
		AgentRuntimeId: aws.String(id),
		EndpointName:   opt.EndpointName,
	}); err != nil {
		var nfe *types.ResourceNotFoundException
		var ade *types.AccessDeniedException
		if !errors.As(err, &nfe) && !errors.As(err, &ade) {
			return fmt.Errorf("get agent runtime endpoint: %w", ErrAgentRuntimeNotFound)
		}
		slog.InfoContext(ctx, "creating agent runtime endpoint", "name", endpointName, "version", version)
		if opt.DryRun {
			slog.DebugContext(ctx, "dry run: create agent runtime endpoint skipped")
			return nil
		}
		resp, err := app.ctrlClient.CreateAgentRuntimeEndpoint(ctx, &bedrockagentcorecontrol.CreateAgentRuntimeEndpointInput{
			AgentRuntimeId:      aws.String(id),
			Name:                aws.String(endpointName),
			AgentRuntimeVersion: aws.String(version),
			Description:         aws.String(fmt.Sprintf("Managed by %s", AppName)),
		})
		if err != nil {
			return fmt.Errorf("CreateAgentRuntimeEndpoint: %w", err)
		}
		slog.DebugContext(ctx, "created agent runtime endpoint", "name", endpointName, "arn", aws.ToString(resp.AgentRuntimeEndpointArn))
	} else {
		slog.InfoContext(ctx, "updating agent runtime endpoint", "name", endpointName, "version", version)
		if opt.DryRun {
			slog.DebugContext(ctx, "dry run: update agent runtime endpoint skipped")
			return nil
		}
		resp, err := app.ctrlClient.UpdateAgentRuntimeEndpoint(ctx, &bedrockagentcorecontrol.UpdateAgentRuntimeEndpointInput{
			AgentRuntimeId:      aws.String(id),
			EndpointName:        aws.String(endpointName),
			AgentRuntimeVersion: aws.String(version),
			Description:         coalesce(current.Description, aws.String(fmt.Sprintf("Managed by %s", AppName))),
		})
		if err != nil {
			return fmt.Errorf("UpdateAgentRuntimeEndpoint: %w", err)
		}
		slog.DebugContext(ctx, "updated agent runtime endpoint", "name", endpointName, "arn", aws.ToString(resp.AgentRuntimeEndpointArn))
	}
	return nil
}
