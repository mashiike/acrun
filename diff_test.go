package acrun

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDiff_NoDifference(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	mockClient := NewMockBedrockAgentCoreClient(ctrl)
	mockECRClient := NewMockECRClient(ctrl)
	mockSTSClient := NewMockSTSClient(ctrl)

	// Mock ListAgentRuntimes response (called by GetAgentRuntimeIDByName)
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

	// Mock GetAgentRuntimeEndpoint response (called for DEFAULT endpoint)
	mockCtrlClient.EXPECT().
		GetAgentRuntimeEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeEndpointOutput{
			TargetVersion: aws.String("1"),
		}, nil)

	// Mock GetAgentRuntime response with same config as testdata/agent_runtime.json
	mockCtrlClient.EXPECT().
		GetAgentRuntime(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeOutput{
			AgentRuntimeId:      aws.String("test-runtime-id"),
			AgentRuntimeName:    aws.String("hosted_agent_dummy"),
			AgentRuntimeArn:     aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/test-runtime-id"),
			AgentRuntimeVersion: aws.String("1"),
			RoleArn:             aws.String("arn:aws:iam::123456789012:role/service-role/DummyServiceRole"),
			AgentRuntimeArtifact: &types.AgentRuntimeArtifactMemberContainerConfiguration{
				Value: types.ContainerConfiguration{
					ContainerUri: aws.String("123456789012.dkr.ecr.us-west-2.amazonaws.com/acrun/sample-mcp:dev"),
				},
			},
			NetworkConfiguration: &types.NetworkConfiguration{
				NetworkMode: types.NetworkModePublic,
			},
			ProtocolConfiguration: &types.ProtocolConfiguration{
				ServerProtocol: types.ServerProtocolMcp,
			},
			EnvironmentVariables: map[string]string{
				"env": "dev",
			},
			AuthorizerConfiguration: &types.AuthorizerConfigurationMemberCustomJWTAuthorizer{
				Value: types.CustomJWTAuthorizerConfiguration{
					DiscoveryUrl: aws.String("https://example.com/.well-known/openid-configuration"),
					AllowedAudience: []string{
						"example_audience",
					},
					AllowedClients: []string{
						"example_client",
					},
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

	opt := &DiffOption{}
	err = app.Diff(context.Background(), opt)
	require.NoError(t, err)
}

func TestDiff_WithDifference(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	mockClient := NewMockBedrockAgentCoreClient(ctrl)
	mockECRClient := NewMockECRClient(ctrl)
	mockSTSClient := NewMockSTSClient(ctrl)

	// Mock ListAgentRuntimes response
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

	// Mock GetAgentRuntimeEndpoint response
	mockCtrlClient.EXPECT().
		GetAgentRuntimeEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeEndpointOutput{
			TargetVersion: aws.String("1"),
		}, nil)

	// Mock GetAgentRuntime response with different roleArn
	mockCtrlClient.EXPECT().
		GetAgentRuntime(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeOutput{
			AgentRuntimeId:      aws.String("test-runtime-id"),
			AgentRuntimeName:    aws.String("hosted_agent_dummy"),
			AgentRuntimeArn:     aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/test-runtime-id"),
			AgentRuntimeVersion: aws.String("1"),
			RoleArn:             aws.String("arn:aws:iam::123456789012:role/DifferentRole"), // Different from local
			AgentRuntimeArtifact: &types.AgentRuntimeArtifactMemberContainerConfiguration{
				Value: types.ContainerConfiguration{
					ContainerUri: aws.String("123456789012.dkr.ecr.us-west-2.amazonaws.com/acrun/sample-mcp:dev"),
				},
			},
			NetworkConfiguration: &types.NetworkConfiguration{
				NetworkMode: types.NetworkModePublic,
			},
			ProtocolConfiguration: &types.ProtocolConfiguration{
				ServerProtocol: types.ServerProtocolMcp,
			},
			EnvironmentVariables: map[string]string{
				"env": "dev",
			},
			AuthorizerConfiguration: &types.AuthorizerConfigurationMemberCustomJWTAuthorizer{
				Value: types.CustomJWTAuthorizerConfiguration{
					DiscoveryUrl: aws.String("https://example.com/.well-known/openid-configuration"),
					AllowedAudience: []string{
						"example_audience",
					},
					AllowedClients: []string{
						"example_client",
					},
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

	opt := &DiffOption{}
	err = app.Diff(context.Background(), opt)
	require.NoError(t, err)

	// Note: diff output goes to os.Stdout via fmt.Print, not app.stdout
	// So we can't capture it in tests, but we can verify no error occurred
}

func TestDiff_RemoteNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	mockClient := NewMockBedrockAgentCoreClient(ctrl)
	mockECRClient := NewMockECRClient(ctrl)
	mockSTSClient := NewMockSTSClient(ctrl)

	// Mock ListAgentRuntimes response (empty - not found)
	mockCtrlClient.EXPECT().
		ListAgentRuntimes(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.ListAgentRuntimesOutput{
			AgentRuntimes: []types.AgentRuntime{},
			NextToken:     nil,
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

	opt := &DiffOption{}
	err = app.Diff(context.Background(), opt)
	require.NoError(t, err)

	// Note: diff output goes to os.Stdout via fmt.Print, not app.stdout
	// So we can't capture it in tests, but we can verify no error occurred
}

func TestDiff_WithExitCode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	mockClient := NewMockBedrockAgentCoreClient(ctrl)
	mockECRClient := NewMockECRClient(ctrl)
	mockSTSClient := NewMockSTSClient(ctrl)

	// Mock ListAgentRuntimes response
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

	// Mock GetAgentRuntimeEndpoint response
	mockCtrlClient.EXPECT().
		GetAgentRuntimeEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeEndpointOutput{
			TargetVersion: aws.String("1"),
		}, nil)

	// Mock GetAgentRuntime response with different roleArn
	mockCtrlClient.EXPECT().
		GetAgentRuntime(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeOutput{
			AgentRuntimeId:      aws.String("test-runtime-id"),
			AgentRuntimeName:    aws.String("hosted_agent_dummy"),
			AgentRuntimeArn:     aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/test-runtime-id"),
			AgentRuntimeVersion: aws.String("1"),
			RoleArn:             aws.String("arn:aws:iam::123456789012:role/DifferentRole"), // Different from local
			AgentRuntimeArtifact: &types.AgentRuntimeArtifactMemberContainerConfiguration{
				Value: types.ContainerConfiguration{
					ContainerUri: aws.String("123456789012.dkr.ecr.us-west-2.amazonaws.com/acrun/sample-mcp:dev"),
				},
			},
			NetworkConfiguration: &types.NetworkConfiguration{
				NetworkMode: types.NetworkModePublic,
			},
			ProtocolConfiguration: &types.ProtocolConfiguration{
				ServerProtocol: types.ServerProtocolMcp,
			},
			EnvironmentVariables: map[string]string{
				"env": "dev",
			},
			AuthorizerConfiguration: &types.AuthorizerConfigurationMemberCustomJWTAuthorizer{
				Value: types.CustomJWTAuthorizerConfiguration{
					DiscoveryUrl: aws.String("https://example.com/.well-known/openid-configuration"),
					AllowedAudience: []string{
						"example_audience",
					},
					AllowedClients: []string{
						"example_client",
					},
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

	opt := &DiffOption{ExitCode: true}
	err = app.Diff(context.Background(), opt)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrDiff))

	// Note: diff output goes to os.Stdout via fmt.Print, not app.stdout
	// So we can't capture it in tests, but we can verify the error occurred
}
