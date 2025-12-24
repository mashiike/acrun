package acrun

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestECRImages(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	mockClient := NewMockBedrockAgentCoreClient(ctrl)
	mockECRClient := NewMockECRClient(ctrl)
	mockSTSClient := NewMockSTSClient(ctrl)

	// Setup expectations: ListAgentRuntimes to resolve name to ID
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

	// Setup expectations: ListAgentRuntimeEndpoints
	mockCtrlClient.EXPECT().
		ListAgentRuntimeEndpoints(gomock.Any(), &bedrockagentcorecontrol.ListAgentRuntimeEndpointsInput{
			AgentRuntimeId: aws.String("test-runtime-id"),
		}).
		Return(&bedrockagentcorecontrol.ListAgentRuntimeEndpointsOutput{
			RuntimeEndpoints: []types.AgentRuntimeEndpoint{
				{
					Id:   aws.String("default-endpoint-id"),
					Name: aws.String("DEFAULT"),
				},
				{
					Id:   aws.String("current-endpoint-id"),
					Name: aws.String("current"),
				},
			},
		}, nil)

	// Setup expectations: GetAgentRuntimeEndpoint for DEFAULT
	mockCtrlClient.EXPECT().
		GetAgentRuntimeEndpoint(gomock.Any(), &bedrockagentcorecontrol.GetAgentRuntimeEndpointInput{
			AgentRuntimeId: aws.String("test-runtime-id"),
			EndpointName:   aws.String("DEFAULT"),
		}).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeEndpointOutput{
			TargetVersion: aws.String("3"),
		}, nil)

	// Setup expectations: GetAgentRuntimeEndpoint for current
	mockCtrlClient.EXPECT().
		GetAgentRuntimeEndpoint(gomock.Any(), &bedrockagentcorecontrol.GetAgentRuntimeEndpointInput{
			AgentRuntimeId: aws.String("test-runtime-id"),
			EndpointName:   aws.String("current"),
		}).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeEndpointOutput{
			TargetVersion: aws.String("2"),
		}, nil)

	// Setup expectations: ListAgentRuntimeVersions
	mockCtrlClient.EXPECT().
		ListAgentRuntimeVersions(gomock.Any(), &bedrockagentcorecontrol.ListAgentRuntimeVersionsInput{
			AgentRuntimeId: aws.String("test-runtime-id"),
		}).
		Return(&bedrockagentcorecontrol.ListAgentRuntimeVersionsOutput{
			AgentRuntimes: []types.AgentRuntime{
				{AgentRuntimeVersion: aws.String("1")},
				{AgentRuntimeVersion: aws.String("2")},
				{AgentRuntimeVersion: aws.String("3")},
				{AgentRuntimeVersion: aws.String("4")},
				{AgentRuntimeVersion: aws.String("5")},
			},
		}, nil)

	// Setup expectations: GetAgentRuntime for each version
	// Version 3 (from DEFAULT endpoint)
	mockCtrlClient.EXPECT().
		GetAgentRuntime(gomock.Any(), &bedrockagentcorecontrol.GetAgentRuntimeInput{
			AgentRuntimeId:      aws.String("test-runtime-id"),
			AgentRuntimeVersion: aws.String("3"),
		}).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeOutput{
			AgentRuntimeArtifact: &types.AgentRuntimeArtifactMemberContainerConfiguration{
				Value: types.ContainerConfiguration{
					ContainerUri: aws.String("123456789012.dkr.ecr.us-west-2.amazonaws.com/my-agent:v3"),
				},
			},
		}, nil).AnyTimes()

	// Version 2 (from current endpoint)
	mockCtrlClient.EXPECT().
		GetAgentRuntime(gomock.Any(), &bedrockagentcorecontrol.GetAgentRuntimeInput{
			AgentRuntimeId:      aws.String("test-runtime-id"),
			AgentRuntimeVersion: aws.String("2"),
		}).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeOutput{
			AgentRuntimeArtifact: &types.AgentRuntimeArtifactMemberContainerConfiguration{
				Value: types.ContainerConfiguration{
					ContainerUri: aws.String("123456789012.dkr.ecr.us-west-2.amazonaws.com/my-agent:v2"),
				},
			},
		}, nil).AnyTimes()

	// Version 5 (from recent versions)
	mockCtrlClient.EXPECT().
		GetAgentRuntime(gomock.Any(), &bedrockagentcorecontrol.GetAgentRuntimeInput{
			AgentRuntimeId:      aws.String("test-runtime-id"),
			AgentRuntimeVersion: aws.String("5"),
		}).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeOutput{
			AgentRuntimeArtifact: &types.AgentRuntimeArtifactMemberContainerConfiguration{
				Value: types.ContainerConfiguration{
					ContainerUri: aws.String("123456789012.dkr.ecr.us-west-2.amazonaws.com/my-agent:v5"),
				},
			},
		}, nil).AnyTimes()

	// Version 4 (from recent versions)
	mockCtrlClient.EXPECT().
		GetAgentRuntime(gomock.Any(), &bedrockagentcorecontrol.GetAgentRuntimeInput{
			AgentRuntimeId:      aws.String("test-runtime-id"),
			AgentRuntimeVersion: aws.String("4"),
		}).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeOutput{
			AgentRuntimeArtifact: &types.AgentRuntimeArtifactMemberContainerConfiguration{
				Value: types.ContainerConfiguration{
					ContainerUri: aws.String("123456789012.dkr.ecr.us-west-2.amazonaws.com/my-agent:v4"),
				},
			},
		}, nil).AnyTimes()

	// Version 1 (from recent versions)
	mockCtrlClient.EXPECT().
		GetAgentRuntime(gomock.Any(), &bedrockagentcorecontrol.GetAgentRuntimeInput{
			AgentRuntimeId:      aws.String("test-runtime-id"),
			AgentRuntimeVersion: aws.String("1"),
		}).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeOutput{
			AgentRuntimeArtifact: &types.AgentRuntimeArtifactMemberContainerConfiguration{
				Value: types.ContainerConfiguration{
					ContainerUri: aws.String("123456789012.dkr.ecr.us-west-2.amazonaws.com/my-agent:v1"),
				},
			},
		}, nil).AnyTimes()

	var stdout, stderr bytes.Buffer
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
	app.SetOutput(&stdout, &stderr)

	err = app.ECRImages(context.Background(), &ECRImagesOption{
		Versions: 5,
	})
	require.NoError(t, err)

	// Parse the output
	var images []string
	err = json.Unmarshal(stdout.Bytes(), &images)
	require.NoError(t, err)

	// Check that we have the expected images (sorted and deduplicated)
	expected := []string{
		"123456789012.dkr.ecr.us-west-2.amazonaws.com/my-agent:v1",
		"123456789012.dkr.ecr.us-west-2.amazonaws.com/my-agent:v2",
		"123456789012.dkr.ecr.us-west-2.amazonaws.com/my-agent:v3",
		"123456789012.dkr.ecr.us-west-2.amazonaws.com/my-agent:v4",
		"123456789012.dkr.ecr.us-west-2.amazonaws.com/my-agent:v5",
	}
	require.Equal(t, expected, images)
}

func TestECRImages_KeepVersionsZero(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	mockClient := NewMockBedrockAgentCoreClient(ctrl)
	mockECRClient := NewMockECRClient(ctrl)
	mockSTSClient := NewMockSTSClient(ctrl)

	// Setup expectations: ListAgentRuntimes to resolve name to ID
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

	// Setup expectations: ListAgentRuntimeEndpoints
	mockCtrlClient.EXPECT().
		ListAgentRuntimeEndpoints(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.ListAgentRuntimeEndpointsOutput{
			RuntimeEndpoints: []types.AgentRuntimeEndpoint{
				{
					Id:   aws.String("default-endpoint-id"),
					Name: aws.String("DEFAULT"),
				},
			},
		}, nil)

	// Setup expectations: GetAgentRuntimeEndpoint for DEFAULT
	mockCtrlClient.EXPECT().
		GetAgentRuntimeEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeEndpointOutput{
			TargetVersion: aws.String("1"),
		}, nil)

	// Setup expectations: GetAgentRuntime for version 1
	mockCtrlClient.EXPECT().
		GetAgentRuntime(gomock.Any(), &bedrockagentcorecontrol.GetAgentRuntimeInput{
			AgentRuntimeId:      aws.String("test-runtime-id"),
			AgentRuntimeVersion: aws.String("1"),
		}).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeOutput{
			AgentRuntimeArtifact: &types.AgentRuntimeArtifactMemberContainerConfiguration{
				Value: types.ContainerConfiguration{
					ContainerUri: aws.String("123456789012.dkr.ecr.us-west-2.amazonaws.com/my-agent:v1"),
				},
			},
		}, nil)

	var stdout, stderr bytes.Buffer
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
	app.SetOutput(&stdout, &stderr)

	// Versions=0 means only collect from endpoints, not from version history
	err = app.ECRImages(context.Background(), &ECRImagesOption{
		Versions: 0,
	})
	require.NoError(t, err)

	var images []string
	err = json.Unmarshal(stdout.Bytes(), &images)
	require.NoError(t, err)

	expected := []string{
		"123456789012.dkr.ecr.us-west-2.amazonaws.com/my-agent:v1",
	}
	require.Equal(t, expected, images)
}
