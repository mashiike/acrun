package acrun

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Songmu/prompter"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types"
)

type DeleteOption struct {
	DryRun          bool          `name:"dry-run" help:"dry run" default:"false"`
	Force           bool          `name:"force" help:"force delete without confirmation" default:"false"`
	WaitDuration    time.Duration `name:"wait-duration" help:"maximum duration to wait until the agent runtime is ready" default:"30m"`
	PollingInterval time.Duration `name:"polling-interval" help:"polling interval to check the agent runtime status" default:"5s"`
}

func (app *App) Delete(ctx context.Context, opt *DeleteOption) error {
	if opt.DryRun {
		slog.WarnContext(ctx, "starting delete in DRY RUN mode. No changes will be made.")
		defer slog.WarnContext(ctx, "ended delete in DRY RUN mode. No changes were made.")
	}

	agentRuntime, err := app.loadAgentRuntimeFile(ctx)
	if err != nil {
		return fmt.Errorf("load agent runtime file: %w", err)
	}

	id, err := app.GetAgentRuntimeIDByName(ctx, *agentRuntime.AgentRuntimeName)
	if err != nil {
		if errors.Is(err, ErrAgentRuntimeNotFound) {
			slog.InfoContext(ctx, "agent runtime not found, nothing to delete", "name", *agentRuntime.AgentRuntimeName)
			return nil
		}
		return fmt.Errorf("get agent runtime ID: %w", err)
	}

	if !opt.Force {
		// Prompt for confirmation
		ok := prompter.YN(fmt.Sprintf("Are you sure you want to delete agent runtime '%s' (ID: %s)?", *agentRuntime.AgentRuntimeName, id), false)
		if !ok {
			slog.InfoContext(ctx, "delete cancelled by user")
			return nil
		}
	}
	var wg sync.WaitGroup
	slog.InfoContext(ctx, "deliting agent runtime endpoints associated with the agent runtime", "name", *agentRuntime.AgentRuntimeName, "id", id)
	p := bedrockagentcorecontrol.NewListAgentRuntimeEndpointsPaginator(
		app.ctrlClient,
		&bedrockagentcorecontrol.ListAgentRuntimeEndpointsInput{
			AgentRuntimeId: aws.String(id),
		},
	)
	for p.HasMorePages() {
		out, err := p.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("ListAgentRuntimeEndpoints: %w", err)
		}
		for _, endpoint := range out.RuntimeEndpoints {
			if aws.ToString(endpoint.Name) == DefaultEndpointName {
				slog.InfoContext(ctx, "skipping deletion of DEFAULT endpoint", "id", aws.ToString(endpoint.Id))
				continue
			}
			wg.Add(1)
			go func(endpoint types.AgentRuntimeEndpoint) {
				defer wg.Done()
				slog.InfoContext(ctx, "deleting agent runtime endpoint", "name", aws.ToString(endpoint.Name), "id", aws.ToString(endpoint.Id))
				input := &bedrockagentcorecontrol.DeleteAgentRuntimeEndpointInput{
					AgentRuntimeId: aws.String(id),
					EndpointName:   endpoint.Name,
				}
				app.DumpIfVerbose(ctx, "DeleteAgentRuntimeEndpointInput", input)
				if opt.DryRun {
					slog.InfoContext(ctx, "dry run: delete agent runtime endpoint skipped", "name", aws.ToString(endpoint.Name), "id", aws.ToString(endpoint.Id))
					return
				}
				_, err := app.ctrlClient.DeleteAgentRuntimeEndpoint(ctx, input)
				if err != nil {
					var nfe *types.ResourceNotFoundException
					if errors.As(err, &nfe) {
						slog.InfoContext(ctx, "agent runtime endpoint already deleted", "name", aws.ToString(endpoint.Name), "id", aws.ToString(endpoint.Id))
						return
					}
					slog.ErrorContext(ctx, "failed to delete agent runtime endpoint", "name", aws.ToString(endpoint.Name), "id", aws.ToString(endpoint.Id), "error", err)
					return
				}
				slog.InfoContext(ctx, "deleted agent runtime endpoint", "name", aws.ToString(endpoint.Name), "id", aws.ToString(endpoint.Id))
				// wait for delete completion
				waiter := &Waiter{
					MaxDuration:   opt.WaitDuration,
					CheckInterval: opt.PollingInterval,
					LogMessage:    "waiting for agent runtime endpoint to be deleted",
					LogAttributes: []any{
						"name", aws.ToString(endpoint.Name),
						"id", aws.ToString(endpoint.Id),
					},
					Checker: func(ctx context.Context) ([]any, bool, error) {
						e, err := app.ctrlClient.GetAgentRuntimeEndpoint(ctx, &bedrockagentcorecontrol.GetAgentRuntimeEndpointInput{
							AgentRuntimeId: aws.String(id),
							EndpointName:   endpoint.Name,
						})
						if err != nil {
							var nfe *types.ResourceNotFoundException
							if errors.As(err, &nfe) {
								return nil, true, nil
							}
							return nil, false, fmt.Errorf("GetAgentRuntimeEndpoint: %w", err)
						}
						return []any{"status", e.Status}, false, nil
					},
				}
				err = waiter.Wait(ctx)
				if err != nil {
					slog.ErrorContext(ctx, "failed to wait for agent runtime endpoint deletion", "name", aws.ToString(endpoint.Name), "id", aws.ToString(endpoint.Id), "error", err)
					return
				}
				slog.InfoContext(ctx, "agent runtime endpoint deleted", "name", aws.ToString(endpoint.Name), "id", aws.ToString(endpoint.Id))
			}(endpoint)
		}
	}
	wg.Wait()
	slog.InfoContext(ctx, "deleting agent runtime", "name", *agentRuntime.AgentRuntimeName, "id", id)
	input := &bedrockagentcorecontrol.DeleteAgentRuntimeInput{
		AgentRuntimeId: aws.String(id),
	}
	app.DumpIfVerbose(ctx, "DeleteAgentRuntimeInput", input)
	if opt.DryRun {
		slog.DebugContext(ctx, "dry run: delete agent runtime skipped")
		return nil
	}
	_, err = app.ctrlClient.DeleteAgentRuntime(ctx, input)
	if err != nil {
		return fmt.Errorf("DeleteAgentRuntime: %w", err)
	}
	slog.InfoContext(ctx, "deleted agent runtime", "name", *agentRuntime.AgentRuntimeName, "id", id)
	return nil
}
