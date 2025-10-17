package acrun

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
)

type RollbackOption struct {
	DryRun       bool    `name:"dry-run" help:"dry run" default:"false"`
	EndpointName *string `name:"endpoint-name" help:"the endpoint name to rollback. if not specified, use the default endpoint."`
	Version      *string `name:"version" help:"the version to rollback to. if not specified, rollback to current version - 1"`
}

func (app *App) Rollback(ctx context.Context, opt *RollbackOption) error {
	e := fillEndpointName(opt.EndpointName)
	if e == DefaultEndpointName {
		return errors.New("rollback of the DEFAULT endpoint is not allowed")
	}
	opt.EndpointName = &e

	if opt.DryRun {
		slog.WarnContext(ctx, "starting rollback in DRY RUN mode. No changes will be made.")
		defer slog.WarnContext(ctx, "ended rollback in DRY RUN mode. No changes were made.")
	}

	agentRuntime, err := app.loadAgentRuntimeFile(ctx)
	if err != nil {
		return fmt.Errorf("load agent runtime file: %w", err)
	}

	id, err := app.GetAgentRuntimeIDByName(ctx, *agentRuntime.AgentRuntimeName)
	if err != nil {
		return fmt.Errorf("get agent runtime ID: %w", err)
	}

	// Get current endpoint
	currentEndpoint, err := app.ctrlClient.GetAgentRuntimeEndpoint(ctx, &bedrockagentcorecontrol.GetAgentRuntimeEndpointInput{
		AgentRuntimeId: aws.String(id),
		EndpointName:   opt.EndpointName,
	})
	if err != nil {
		return fmt.Errorf("GetAgentRuntimeEndpoint: %w", err)
	}

	currentVersionStr := aws.ToString(currentEndpoint.TargetVersion)

	// Determine target version
	var targetVersion string
	if opt.Version != nil {
		targetVersion = *opt.Version
	} else {
		// Automatic rollback: current version - 1
		var currentVersionInt int
		_, err := fmt.Sscanf(currentVersionStr, "%d", &currentVersionInt)
		if err != nil {
			return fmt.Errorf("failed to parse current version '%s' as integer: %w", currentVersionStr, err)
		}
		if currentVersionInt <= 1 {
			return fmt.Errorf("cannot rollback: current version is %d (minimum is 1)", currentVersionInt)
		}
		targetVersion = fmt.Sprintf("%d", currentVersionInt-1)
		slog.InfoContext(ctx, "automatic rollback", "current", currentVersionStr, "target", targetVersion)
	}

	// Verify the target version exists
	versionsOutput, err := app.ctrlClient.ListAgentRuntimeVersions(ctx, &bedrockagentcorecontrol.ListAgentRuntimeVersionsInput{
		AgentRuntimeId: aws.String(id),
	})
	if err != nil {
		return fmt.Errorf("ListAgentRuntimeVersions: %w", err)
	}

	versionExists := false
	for _, v := range versionsOutput.AgentRuntimes {
		if aws.ToString(v.AgentRuntimeVersion) == targetVersion {
			versionExists = true
			break
		}
	}
	if !versionExists {
		return fmt.Errorf("version %s not found", targetVersion)
	}

	if currentVersionStr == targetVersion {
		slog.InfoContext(ctx, "endpoint is already at the specified version", "endpoint", *opt.EndpointName, "version", targetVersion)
		return nil
	}

	slog.InfoContext(ctx, "rolling back endpoint", "endpoint", *opt.EndpointName, "from", currentVersionStr, "to", targetVersion)
	if opt.DryRun {
		slog.DebugContext(ctx, "dry run: rollback endpoint skipped")
		return nil
	}

	_, err = app.ctrlClient.UpdateAgentRuntimeEndpoint(ctx, &bedrockagentcorecontrol.UpdateAgentRuntimeEndpointInput{
		AgentRuntimeId:      aws.String(id),
		EndpointName:        opt.EndpointName,
		AgentRuntimeVersion: aws.String(targetVersion),
		Description:         currentEndpoint.Description,
	})
	if err != nil {
		return fmt.Errorf("UpdateAgentRuntimeEndpoint: %w", err)
	}

	slog.InfoContext(ctx, "rolled back endpoint", "endpoint", *opt.EndpointName, "version", targetVersion)
	return nil
}
