package acrun

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types"
)

// ECRImagesOption represents the options for the ecr-images command.
type ECRImagesOption struct {
	Versions int `help:"Number of recent versions to include." default:"5"`
}

// ECRImages retrieves the list of ECR image URIs used by the AgentRuntime.
// This includes images from all endpoints (DEFAULT and aliases) and recent N versions.
func (app *App) ECRImages(ctx context.Context, opt *ECRImagesOption) error {
	agentRuntime, err := app.loadAgentRuntimeFile(ctx)
	if err != nil {
		return fmt.Errorf("load agent runtime file: %w", err)
	}

	agentRuntimeName := *agentRuntime.AgentRuntimeName
	slog.DebugContext(ctx, "starting ecr-images",
		"agent_runtime_name", agentRuntimeName,
		"versions", opt.Versions,
	)

	id, err := app.GetAgentRuntimeIDByName(ctx, agentRuntimeName)
	if err != nil {
		return fmt.Errorf("get agent runtime ID: %w", err)
	}

	images := make(map[string]struct{})

	if err := app.collectImagesFromEndpoints(ctx, id, images); err != nil {
		return err
	}

	if opt.Versions > 0 {
		if err := app.collectImagesFromVersions(ctx, id, opt.Versions, images); err != nil {
			return err
		}
	}

	result := make([]string, 0, len(images))
	for img := range images {
		result = append(result, img)
	}
	sort.Strings(result)

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	fmt.Fprintln(app.stdout, string(output))
	return nil
}

func (app *App) collectImagesFromEndpoints(ctx context.Context, agentRuntimeID string, images map[string]struct{}) error {
	slog.DebugContext(ctx, "collecting images from endpoints", "agent_runtime_id", agentRuntimeID)

	endpointsOutput, err := app.ctrlClient.ListAgentRuntimeEndpoints(ctx, &bedrockagentcorecontrol.ListAgentRuntimeEndpointsInput{
		AgentRuntimeId: aws.String(agentRuntimeID),
	})
	if err != nil {
		return fmt.Errorf("ListAgentRuntimeEndpoints: %w", err)
	}

	versions := make(map[string]struct{})
	for _, endpoint := range endpointsOutput.RuntimeEndpoints {
		endpointName := aws.ToString(endpoint.Name)
		// ListAgentRuntimeEndpoints does not include TargetVersion, so we need to fetch each endpoint's details
		endpointDetail, err := app.ctrlClient.GetAgentRuntimeEndpoint(ctx, &bedrockagentcorecontrol.GetAgentRuntimeEndpointInput{
			AgentRuntimeId: aws.String(agentRuntimeID),
			EndpointName:   aws.String(endpointName),
		})
		if err != nil {
			slog.WarnContext(ctx, "failed to get endpoint details",
				"endpoint", endpointName,
				"error", err,
			)
			continue
		}

		var version string
		if endpointDetail.TargetVersion != nil {
			version = *endpointDetail.TargetVersion
		} else if endpointDetail.LiveVersion != nil {
			version = *endpointDetail.LiveVersion
		}

		if version != "" {
			versions[version] = struct{}{}
			slog.DebugContext(ctx, "found endpoint",
				"name", endpointName,
				"version", version,
			)
		}
	}

	for version := range versions {
		uri, err := app.getContainerURIForVersion(ctx, agentRuntimeID, version)
		if err != nil {
			slog.WarnContext(ctx, "failed to get container URI for version",
				"version", version,
				"error", err,
			)
			continue
		}
		if uri != "" {
			images[uri] = struct{}{}
		}
	}

	return nil
}

func (app *App) collectImagesFromVersions(ctx context.Context, agentRuntimeID string, versions int, images map[string]struct{}) error {
	slog.DebugContext(ctx, "collecting images from versions",
		"agent_runtime_id", agentRuntimeID,
		"versions", versions,
	)

	versionsOutput, err := app.ctrlClient.ListAgentRuntimeVersions(ctx, &bedrockagentcorecontrol.ListAgentRuntimeVersionsInput{
		AgentRuntimeId: aws.String(agentRuntimeID),
	})
	if err != nil {
		return fmt.Errorf("ListAgentRuntimeVersions: %w", err)
	}

	sortedVersions := make([]string, 0, len(versionsOutput.AgentRuntimes))
	for _, v := range versionsOutput.AgentRuntimes {
		if v.AgentRuntimeVersion != nil {
			sortedVersions = append(sortedVersions, *v.AgentRuntimeVersion)
		}
	}
	slices.SortFunc(sortedVersions, func(a, b string) int {
		// descending order to get the newest versions first
		var va, vb int
		fmt.Sscanf(a, "%d", &va) //nolint:errcheck
		fmt.Sscanf(b, "%d", &vb) //nolint:errcheck
		return vb - va
	})

	if len(sortedVersions) > versions {
		sortedVersions = sortedVersions[:versions]
	}

	slog.DebugContext(ctx, "versions to collect", "versions", sortedVersions)

	for _, version := range sortedVersions {
		uri, err := app.getContainerURIForVersion(ctx, agentRuntimeID, version)
		if err != nil {
			slog.WarnContext(ctx, "failed to get container URI for version",
				"version", version,
				"error", err,
			)
			continue
		}
		if uri != "" {
			images[uri] = struct{}{}
		}
	}

	return nil
}

func (app *App) getContainerURIForVersion(ctx context.Context, agentRuntimeID, version string) (string, error) {
	resp, err := app.ctrlClient.GetAgentRuntime(ctx, &bedrockagentcorecontrol.GetAgentRuntimeInput{
		AgentRuntimeId:      aws.String(agentRuntimeID),
		AgentRuntimeVersion: aws.String(version),
	})
	if err != nil {
		return "", fmt.Errorf("GetAgentRuntime: %w", err)
	}

	return extractContainerURI(resp.AgentRuntimeArtifact), nil
}

func extractContainerURI(artifact types.AgentRuntimeArtifact) string {
	if artifact == nil {
		return ""
	}
	switch v := artifact.(type) {
	case *types.AgentRuntimeArtifactMemberContainerConfiguration:
		return aws.ToString(v.Value.ContainerUri)
	default:
		return ""
	}
}
