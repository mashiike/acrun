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

func TestDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	mockClient := NewMockBedrockAgentCoreClient(ctrl)
	mockECRClient := NewMockECRClient(ctrl)
	mockSTSClient := NewMockSTSClient(ctrl)

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
		ListAgentRuntimeEndpoints(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.ListAgentRuntimeEndpointsOutput{
			RuntimeEndpoints: []types.AgentRuntimeEndpoint{
				{
					Id:   aws.String("current"),
					Name: aws.String("current"),
				},
				{
					Id:   aws.String(DefaultEndpointName),
					Name: aws.String(DefaultEndpointName),
				},
			},
		}, nil)
	mockCtrlClient.EXPECT().
		DeleteAgentRuntime(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.DeleteAgentRuntimeOutput{}, nil)
	mockCtrlClient.EXPECT().
		DeleteAgentRuntimeEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.DeleteAgentRuntimeEndpointOutput{}, nil)
	gomock.InOrder(
		mockCtrlClient.EXPECT().
			GetAgentRuntimeEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&bedrockagentcorecontrol.GetAgentRuntimeEndpointOutput{
				Id:     aws.String("current"),
				Name:   aws.String("current"),
				Status: types.AgentRuntimeEndpointStatusDeleting,
			}, nil).Times(1),
		mockCtrlClient.EXPECT().
			GetAgentRuntimeEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, &types.ResourceNotFoundException{}).Times(1),
	)
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

	opt := &DeleteOption{
		DryRun: false,
		Force:  true, // Skip confirmation
	}

	err = app.Delete(context.Background(), opt)
	require.NoError(t, err)
}

func TestDelete_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	mockClient := NewMockBedrockAgentCoreClient(ctrl)
	mockECRClient := NewMockECRClient(ctrl)
	mockSTSClient := NewMockSTSClient(ctrl)

	// Setup expectations - no runtime found
	mockCtrlClient.EXPECT().
		ListAgentRuntimes(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.ListAgentRuntimesOutput{
			AgentRuntimes: []types.AgentRuntime{},
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

	opt := &DeleteOption{
		DryRun: false,
		Force:  true,
	}

	err = app.Delete(context.Background(), opt)
	require.NoError(t, err) // Should succeed even if not found
}

func TestDelete_DryRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	mockClient := NewMockBedrockAgentCoreClient(ctrl)
	mockECRClient := NewMockECRClient(ctrl)
	mockSTSClient := NewMockSTSClient(ctrl)

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
		}, nil)
	mockCtrlClient.EXPECT().
		ListAgentRuntimeEndpoints(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.ListAgentRuntimeEndpointsOutput{
			RuntimeEndpoints: []types.AgentRuntimeEndpoint{
				{
					Id:   aws.String("current"),
					Name: aws.String("current"),
				},
				{
					Id:   aws.String(DefaultEndpointName),
					Name: aws.String(DefaultEndpointName),
				},
			},
		}, nil)
	// DeleteAgentRuntime should NOT be called in dry-run mode

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

	opt := &DeleteOption{
		DryRun: true,
		Force:  true,
	}

	err = app.Delete(context.Background(), opt)
	require.NoError(t, err)
}
