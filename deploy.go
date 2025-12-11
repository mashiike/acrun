package acrun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types"
)

type DeployOption struct {
	DryRun          bool          `name:"dry-run" help:"dry run" default:"false"`
	EndpointName    *string       `name:"endpoint-name" help:"the endpoint name to deploy. if not specified, use the default endpoint."`
	WaitDuration    time.Duration `name:"wait-duration" help:"maximum duration to wait until the agent runtime is ready" default:"30m"`
	PollingInterval time.Duration `name:"polling-interval" help:"polling interval to check the agent runtime status" default:"5s"`
}

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
	id, err := app.GetAgentRuntimeIDByName(ctx, *agentRuntime.AgentRuntimeName)
	if err != nil {
		if !errors.Is(err, ErrAgentRuntimeNotFound) {
			return fmt.Errorf("get agent runtime ID by name: %w", err)
		}
		id, version, err = app.createRuntimeAgent(ctx, agentRuntime, opt)
		if err != nil {
			return fmt.Errorf("createRuntimeAgent: %w", err)
		}
	} else {
		version, err = app.updateRuntimeAgent(ctx, agentRuntime, opt)
		if err != nil {
			return fmt.Errorf("updateRuntimeAgent: %w", err)
		}
	}
	if err := app.waitRuntimeAgentReady(ctx, id, version, opt); err != nil {
		return fmt.Errorf("waitRuntimeAgentReady: %w", err)
	}
	slog.InfoContext(ctx, "deployed agent runtime", "name", aws.ToString(agentRuntime.AgentRuntimeName), "id", id, "version", version)
	if err := app.createOrUpdateAgentRuntimeEndpoint(ctx, id, *opt.EndpointName, version, opt); err != nil {
		return fmt.Errorf("createOrUpdateAgentRuntimeEndpoint: %w", err)
	}
	return nil
}

func (app *App) createRuntimeAgent(ctx context.Context, agentRuntime *AgentRuntime, opt *DeployOption) (string, string, error) {
	slog.InfoContext(ctx, "creating agent runtime", "name", aws.ToString(agentRuntime.AgentRuntimeName))
	if app.verbose {
		bs, err := json.MarshalIndent(agentRuntime, "", "  ")
		if err != nil {
			return "", "", fmt.Errorf("marshal create input to json: %w", err)
		}
		fmt.Fprintf(app.stderr, "CreateAgentRuntimeInput: %s\n", string(bs))
	}
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
	if app.verbose {
		bs, err := json.MarshalIndent(input, "", "  ")
		if err != nil {
			return "", fmt.Errorf("marshal update input to json: %w", err)
		}
		fmt.Fprintf(app.stderr, "UpdateAgentRuntimeInput: %s\n", string(bs))
	}
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

func (app *App) waitRuntimeAgentReady(ctx context.Context, id string, version string, opt *DeployOption) error {
	if opt.DryRun {
		slog.DebugContext(ctx, "dry run: wait for agent runtime to be ready skipped")
		return nil
	}
	slog.InfoContext(ctx, "waiting for agent runtime to be ready", "id", id, "version", version)
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, opt.WaitDuration)
	defer cancel()
	out, err := app.ctrlClient.GetAgentRuntime(ctx, &bedrockagentcorecontrol.GetAgentRuntimeInput{
		AgentRuntimeId:      aws.String(id),
		AgentRuntimeVersion: aws.String(version),
	})
	if err != nil {
		return fmt.Errorf("GetAgentRuntime: %w", err)
	}
	tick := time.NewTicker(opt.PollingInterval)
	defer tick.Stop()
	for {
		if out.Status == types.AgentRuntimeStatusReady {
			slog.InfoContext(ctx, "agent runtime is ready", "id", id, "version", version)
			return nil
		}
		slog.InfoContext(ctx, "agent runtime is not ready yet", "id", id, "version", version, "status", out.Status, "elapsed", time.Since(start).String())
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for agent runtime to be ready: %w", ctx.Err())
		case <-tick.C:
		}
		var err error
		out, err = app.ctrlClient.GetAgentRuntime(ctx, &bedrockagentcorecontrol.GetAgentRuntimeInput{
			AgentRuntimeId:      aws.String(id),
			AgentRuntimeVersion: aws.String(version),
		})
		if err != nil {
			return fmt.Errorf("GetAgentRuntime: %w", err)
		}
	}
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
