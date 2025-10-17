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

func TestDeploy_Create(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	mockClient := NewMockBedrockAgentCoreClient(ctrl)
	mockECRClient := NewMockECRClient(ctrl)
	mockSTSClient := NewMockSTSClient(ctrl)

	endpointName := "test-endpoint"

	// Runtime not found (will create new)
	mockCtrlClient.EXPECT().
		ListAgentRuntimes(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.ListAgentRuntimesOutput{
			AgentRuntimes: []types.AgentRuntime{},
			NextToken:     nil,
		}, nil)

	// Create agent runtime
	mockCtrlClient.EXPECT().
		CreateAgentRuntime(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.CreateAgentRuntimeOutput{
			AgentRuntimeId:      aws.String("new-runtime-id"),
			AgentRuntimeVersion: aws.String("1"),
			AgentRuntimeArn:     aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/new-runtime-id"),
		}, nil)

	// Endpoint not found (will create new)
	mockCtrlClient.EXPECT().
		GetAgentRuntimeEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, &types.ResourceNotFoundException{})

	// Create endpoint
	mockCtrlClient.EXPECT().
		CreateAgentRuntimeEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.CreateAgentRuntimeEndpointOutput{
			AgentRuntimeEndpointArn: aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:endpoint/test"),
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

	opt := &DeployOption{
		DryRun:       false,
		EndpointName: &endpointName,
	}

	err = app.Deploy(context.Background(), opt)
	require.NoError(t, err)
}

func TestDeploy_Update(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	mockClient := NewMockBedrockAgentCoreClient(ctrl)
	mockECRClient := NewMockECRClient(ctrl)
	mockSTSClient := NewMockSTSClient(ctrl)

	endpointName := "test-endpoint"

	// Runtime found (will update)
	mockCtrlClient.EXPECT().
		ListAgentRuntimes(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.ListAgentRuntimesOutput{
			AgentRuntimes: []types.AgentRuntime{
				{
					AgentRuntimeId:   aws.String("existing-runtime-id"),
					AgentRuntimeName: aws.String("hosted_agent_dummy"),
					AgentRuntimeArn:  aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/existing-runtime-id"),
				},
			},
			NextToken: nil,
		}, nil)

	// Get existing runtime for update
	mockCtrlClient.EXPECT().
		GetAgentRuntimeEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeEndpointOutput{
			AgentRuntimeArn: aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/existing-runtime-id"),
			TargetVersion:   aws.String("1"),
		}, nil)

	mockCtrlClient.EXPECT().
		GetAgentRuntime(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeOutput{
			AgentRuntimeId:      aws.String("existing-runtime-id"),
			AgentRuntimeArn:     aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/existing-runtime-id"),
			AgentRuntimeVersion: aws.String("1"),
		}, nil)

	// Update agent runtime
	mockCtrlClient.EXPECT().
		UpdateAgentRuntime(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.UpdateAgentRuntimeOutput{
			AgentRuntimeId:      aws.String("existing-runtime-id"),
			AgentRuntimeVersion: aws.String("2"),
			AgentRuntimeArn:     aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/existing-runtime-id"),
		}, nil)

	// Endpoint exists (will update)
	mockCtrlClient.EXPECT().
		GetAgentRuntimeEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeEndpointOutput{
			TargetVersion: aws.String("1"),
			Description:   aws.String("Existing endpoint"),
		}, nil)

	// Update endpoint
	mockCtrlClient.EXPECT().
		UpdateAgentRuntimeEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.UpdateAgentRuntimeEndpointOutput{
			AgentRuntimeEndpointArn: aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:endpoint/test"),
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

	opt := &DeployOption{
		DryRun:       false,
		EndpointName: &endpointName,
	}

	err = app.Deploy(context.Background(), opt)
	require.NoError(t, err)
}

func TestDeploy_DryRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	mockClient := NewMockBedrockAgentCoreClient(ctrl)
	mockECRClient := NewMockECRClient(ctrl)
	mockSTSClient := NewMockSTSClient(ctrl)

	endpointName := "test-endpoint"

	// Runtime not found (will create new, but dry-run)
	mockCtrlClient.EXPECT().
		ListAgentRuntimes(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.ListAgentRuntimesOutput{
			AgentRuntimes: []types.AgentRuntime{},
			NextToken:     nil,
		}, nil)

	// GetAgentRuntimeEndpoint will be called but no Create/Update should happen
	mockCtrlClient.EXPECT().
		GetAgentRuntimeEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, &types.ResourceNotFoundException{})

	// No CreateAgentRuntime, CreateAgentRuntimeEndpoint calls in dry-run

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

	opt := &DeployOption{
		DryRun:       true,
		EndpointName: &endpointName,
	}

	err = app.Deploy(context.Background(), opt)
	require.NoError(t, err)
}

func TestDeploy_DefaultEndpointRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	mockClient := NewMockBedrockAgentCoreClient(ctrl)
	mockECRClient := NewMockECRClient(ctrl)
	mockSTSClient := NewMockSTSClient(ctrl)

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

	defaultEndpoint := "DEFAULT"
	opt := &DeployOption{
		DryRun:       false,
		EndpointName: &defaultEndpoint,
	}

	err = app.Deploy(context.Background(), opt)
	require.Error(t, err)
	require.Contains(t, err.Error(), "DEFAULT endpoint is not allowed")
}
