package acrun

import (
	"bytes"
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRollback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	mockClient := NewMockBedrockAgentCoreClient(ctrl)
	mockECRClient := NewMockECRClient(ctrl)
	mockSTSClient := NewMockSTSClient(ctrl)

	endpointName := "test-endpoint"
	targetVersion := "2"

	// Setup expectations
	mockCtrlClient.EXPECT().
		ListAgentRuntimes(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.ListAgentRuntimesOutput{
			AgentRuntimes: []types.AgentRuntime{
				{
					AgentRuntimeId:   aws.String("test-runtime-id"),
					AgentRuntimeName: aws.String("hosted_agent_dummy"),
					AgentRuntimeArn:  aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/test-runtime-id"),
				},
			},
			NextToken: nil,
		}, nil)

	mockCtrlClient.EXPECT().
		ListAgentRuntimeVersions(gomock.Any(), &bedrockagentcorecontrol.ListAgentRuntimeVersionsInput{
			AgentRuntimeId: aws.String("test-runtime-id"),
		}).
		Return(&bedrockagentcorecontrol.ListAgentRuntimeVersionsOutput{
			AgentRuntimes: []types.AgentRuntime{
				{
					AgentRuntimeVersion: aws.String("1"),
				},
				{
					AgentRuntimeVersion: aws.String("2"),
				},
				{
					AgentRuntimeVersion: aws.String("3"),
				},
			},
		}, nil)

	mockCtrlClient.EXPECT().
		GetAgentRuntimeEndpoint(gomock.Any(), &bedrockagentcorecontrol.GetAgentRuntimeEndpointInput{
			AgentRuntimeId: aws.String("test-runtime-id"),
			EndpointName:   &endpointName,
		}).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeEndpointOutput{
			TargetVersion: aws.String("3"), // Current version
			Description:   aws.String("Test endpoint"),
		}, nil)

	mockCtrlClient.EXPECT().
		UpdateAgentRuntimeEndpoint(gomock.Any(), &bedrockagentcorecontrol.UpdateAgentRuntimeEndpointInput{
			AgentRuntimeId:      aws.String("test-runtime-id"),
			EndpointName:        &endpointName,
			AgentRuntimeVersion: aws.String(targetVersion),
			Description:         aws.String("Test endpoint"),
		}).
		Return(&bedrockagentcorecontrol.UpdateAgentRuntimeEndpointOutput{}, nil)

	app, err := NewWithClient(
		context.Background(),
		&GlobalOption{AgentRuntime: "testdata/agent_runtime.json"},
		aws.Config{},
		mockCtrlClient,
		mockClient,
		mockECRClient,
		mockSTSClient,
	)
	require.NoError(t, err)
	var stdout, stderr bytes.Buffer
	app.SetOutput(&stdout, &stderr)

	opt := &RollbackOption{
		DryRun:       false,
		EndpointName: &endpointName,
		Version:      &targetVersion,
	}

	err = app.Rollback(context.Background(), opt)
	require.NoError(t, err)
}

func TestRollback_VersionNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	mockClient := NewMockBedrockAgentCoreClient(ctrl)
	mockECRClient := NewMockECRClient(ctrl)
	mockSTSClient := NewMockSTSClient(ctrl)

	endpointName := "test-endpoint"
	targetVersion := "99" // Non-existent version

	// Setup expectations
	mockCtrlClient.EXPECT().
		ListAgentRuntimes(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.ListAgentRuntimesOutput{
			AgentRuntimes: []types.AgentRuntime{
				{
					AgentRuntimeId:   aws.String("test-runtime-id"),
					AgentRuntimeName: aws.String("hosted_agent_dummy"),
					AgentRuntimeArn:  aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/test-runtime-id"),
				},
			},
			NextToken: nil,
		}, nil)

	mockCtrlClient.EXPECT().
		GetAgentRuntimeEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeEndpointOutput{
			TargetVersion: aws.String("3"),
			Description:   aws.String("Test endpoint"),
		}, nil)

	mockCtrlClient.EXPECT().
		ListAgentRuntimeVersions(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.ListAgentRuntimeVersionsOutput{
			AgentRuntimes: []types.AgentRuntime{
				{
					AgentRuntimeVersion: aws.String("1"),
				},
				{
					AgentRuntimeVersion: aws.String("2"),
				},
			},
		}, nil)

	app, err := NewWithClient(
		context.Background(),
		&GlobalOption{AgentRuntime: "testdata/agent_runtime.json"},
		aws.Config{},
		mockCtrlClient,
		mockClient,
		mockECRClient,
		mockSTSClient,
	)
	require.NoError(t, err)
	var stdout, stderr bytes.Buffer
	app.SetOutput(&stdout, &stderr)

	opt := &RollbackOption{
		DryRun:       false,
		EndpointName: &endpointName,
		Version:      &targetVersion,
	}

	err = app.Rollback(context.Background(), opt)
	require.Error(t, err)
	require.Contains(t, err.Error(), "version 99 not found")
}

func TestRollback_AlreadyAtVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	mockClient := NewMockBedrockAgentCoreClient(ctrl)
	mockECRClient := NewMockECRClient(ctrl)
	mockSTSClient := NewMockSTSClient(ctrl)

	endpointName := "test-endpoint"
	targetVersion := "2"

	// Setup expectations
	mockCtrlClient.EXPECT().
		ListAgentRuntimes(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.ListAgentRuntimesOutput{
			AgentRuntimes: []types.AgentRuntime{
				{
					AgentRuntimeId:   aws.String("test-runtime-id"),
					AgentRuntimeName: aws.String("hosted_agent_dummy"),
					AgentRuntimeArn:  aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/test-runtime-id"),
				},
			},
			NextToken: nil,
		}, nil)

	mockCtrlClient.EXPECT().
		ListAgentRuntimeVersions(gomock.Any(), &bedrockagentcorecontrol.ListAgentRuntimeVersionsInput{
			AgentRuntimeId: aws.String("test-runtime-id"),
		}).
		Return(&bedrockagentcorecontrol.ListAgentRuntimeVersionsOutput{
			AgentRuntimes: []types.AgentRuntime{
				{
					AgentRuntimeVersion: aws.String("1"),
				},
				{
					AgentRuntimeVersion: aws.String("2"),
				},
			},
		}, nil)

	mockCtrlClient.EXPECT().
		GetAgentRuntimeEndpoint(gomock.Any(), &bedrockagentcorecontrol.GetAgentRuntimeEndpointInput{
			AgentRuntimeId: aws.String("test-runtime-id"),
			EndpointName:   &endpointName,
		}).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeEndpointOutput{
			TargetVersion: aws.String(targetVersion), // Already at target version
			Description:   aws.String("Test endpoint"),
		}, nil)

	// UpdateAgentRuntimeEndpoint should NOT be called

	app, err := NewWithClient(
		context.Background(),
		&GlobalOption{AgentRuntime: "testdata/agent_runtime.json"},
		aws.Config{},
		mockCtrlClient,
		mockClient,
		mockECRClient,
		mockSTSClient,
	)
	require.NoError(t, err)
	var stdout, stderr bytes.Buffer
	app.SetOutput(&stdout, &stderr)

	opt := &RollbackOption{
		DryRun:       false,
		EndpointName: &endpointName,
		Version:      &targetVersion,
	}

	err = app.Rollback(context.Background(), opt)
	require.NoError(t, err)
}

func TestRollback_DryRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	mockClient := NewMockBedrockAgentCoreClient(ctrl)
	mockECRClient := NewMockECRClient(ctrl)
	mockSTSClient := NewMockSTSClient(ctrl)

	endpointName := "test-endpoint"
	targetVersion := "2"

	// Setup expectations
	mockCtrlClient.EXPECT().
		ListAgentRuntimes(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.ListAgentRuntimesOutput{
			AgentRuntimes: []types.AgentRuntime{
				{
					AgentRuntimeId:   aws.String("test-runtime-id"),
					AgentRuntimeName: aws.String("hosted_agent_dummy"),
					AgentRuntimeArn:  aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/test-runtime-id"),
				},
			},
			NextToken: nil,
		}, nil)

	mockCtrlClient.EXPECT().
		ListAgentRuntimeVersions(gomock.Any(), &bedrockagentcorecontrol.ListAgentRuntimeVersionsInput{
			AgentRuntimeId: aws.String("test-runtime-id"),
		}).
		Return(&bedrockagentcorecontrol.ListAgentRuntimeVersionsOutput{
			AgentRuntimes: []types.AgentRuntime{
				{
					AgentRuntimeVersion: aws.String("1"),
				},
				{
					AgentRuntimeVersion: aws.String("2"),
				},
				{
					AgentRuntimeVersion: aws.String("3"),
				},
			},
		}, nil)

	mockCtrlClient.EXPECT().
		GetAgentRuntimeEndpoint(gomock.Any(), &bedrockagentcorecontrol.GetAgentRuntimeEndpointInput{
			AgentRuntimeId: aws.String("test-runtime-id"),
			EndpointName:   &endpointName,
		}).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeEndpointOutput{
			TargetVersion: aws.String("3"),
			Description:   aws.String("Test endpoint"),
		}, nil)

	// UpdateAgentRuntimeEndpoint should NOT be called in dry-run mode

	app, err := NewWithClient(
		context.Background(),
		&GlobalOption{AgentRuntime: "testdata/agent_runtime.json"},
		aws.Config{},
		mockCtrlClient,
		mockClient,
		mockECRClient,
		mockSTSClient,
	)
	require.NoError(t, err)
	var stdout, stderr bytes.Buffer
	app.SetOutput(&stdout, &stderr)

	opt := &RollbackOption{
		DryRun:       true,
		EndpointName: &endpointName,
		Version:      &targetVersion,
	}

	err = app.Rollback(context.Background(), opt)
	require.NoError(t, err)
}
