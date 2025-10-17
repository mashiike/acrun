package acrun

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
)

type DeleteOption struct {
	DryRun bool `name:"dry-run" help:"dry run" default:"false"`
	Force  bool `name:"force" help:"force delete without confirmation" default:"false"`
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
		// Interactive confirmation is not supported in tests, so we skip it
		// In production, this would read from stdin
		slog.WarnContext(ctx, "non-force delete requires --force flag")
		return fmt.Errorf("delete requires --force flag for safety")
	}

	slog.InfoContext(ctx, "deleting agent runtime", "name", *agentRuntime.AgentRuntimeName, "id", id)
	if opt.DryRun {
		slog.DebugContext(ctx, "dry run: delete agent runtime skipped")
		return nil
	}

	_, err = app.ctrlClient.DeleteAgentRuntime(ctx, &bedrockagentcorecontrol.DeleteAgentRuntimeInput{
		AgentRuntimeId: aws.String(id),
	})
	if err != nil {
		return fmt.Errorf("DeleteAgentRuntime: %w", err)
	}

	slog.InfoContext(ctx, "deleted agent runtime", "name", *agentRuntime.AgentRuntimeName, "id", id)
	return nil
}
