package acrun

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestInit_JSON(t *testing.T) {
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
					AgentRuntimeName: aws.String("test-runtime"),
					AgentRuntimeArn:  aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/test-runtime-id"),
				},
			},
			NextToken: nil,
		}, nil)

	// Mock GetAgentRuntimeEndpoint response (called by GetAgentRuntimeVersionByEndpointName for DEFAULT endpoint)
	mockCtrlClient.EXPECT().
		GetAgentRuntimeEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeEndpointOutput{
			TargetVersion: aws.String("1"),
		}, nil)

	// Mock GetAgentRuntime response
	mockCtrlClient.EXPECT().
		GetAgentRuntime(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeOutput{
			AgentRuntimeId:      aws.String("test-runtime-id"),
			AgentRuntimeName:    aws.String("test-runtime"),
			AgentRuntimeArn:     aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/test-runtime-id"),
			AgentRuntimeVersion: aws.String("1"),
			RoleArn:             aws.String("arn:aws:iam::123456789012:role/test-role"),
			AgentRuntimeArtifact: &types.AgentRuntimeArtifactMemberContainerConfiguration{
				Value: types.ContainerConfiguration{
					ContainerUri: aws.String("123456789012.dkr.ecr.us-west-2.amazonaws.com/test:latest"),
				},
			},
		}, nil)

	// Create temp directory
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tempDir)
	require.NoError(t, err)
	defer os.Chdir(originalWd)

	app, err := NewWithClient(
		context.Background(),
		&GlobalOption{},
		aws.Config{},
		mockCtrlClient,
		mockClient,
		mockECRClient,
		mockSTSClient,
	)
	require.NoError(t, err)

	opt := &InitOption{
		AgentRuntimeName: "test-runtime",
		Format:           "json",
		ForceOverwrite:   true,
	}

	err = app.Init(context.Background(), opt)
	require.NoError(t, err)

	// Verify file was created
	filename := filepath.Join(tempDir, "agent_runtime.json")
	require.FileExists(t, filename)

	// Verify file contents
	content, err := os.ReadFile(filename)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(content, &result)
	require.NoError(t, err)

	// Verify required fields
	require.Contains(t, result, "agentRuntimeName")
	require.Equal(t, "test-runtime", result["agentRuntimeName"])
	require.Contains(t, result, "roleArn")
}

func TestInit_Jsonnet(t *testing.T) {
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
					AgentRuntimeName: aws.String("test-runtime"),
					AgentRuntimeArn:  aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/test-runtime-id"),
				},
			},
			NextToken: nil,
		}, nil)

	// Mock GetAgentRuntimeEndpoint response (called by GetAgentRuntimeVersionByEndpointName for DEFAULT endpoint)
	mockCtrlClient.EXPECT().
		GetAgentRuntimeEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeEndpointOutput{
			TargetVersion: aws.String("1"),
		}, nil)

	// Mock GetAgentRuntime response
	mockCtrlClient.EXPECT().
		GetAgentRuntime(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeOutput{
			AgentRuntimeId:      aws.String("test-runtime-id"),
			AgentRuntimeName:    aws.String("test-runtime"),
			AgentRuntimeArn:     aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/test-runtime-id"),
			AgentRuntimeVersion: aws.String("1"),
			RoleArn:             aws.String("arn:aws:iam::123456789012:role/test-role"),
			AgentRuntimeArtifact: &types.AgentRuntimeArtifactMemberContainerConfiguration{
				Value: types.ContainerConfiguration{
					ContainerUri: aws.String("123456789012.dkr.ecr.us-west-2.amazonaws.com/test:latest"),
				},
			},
		}, nil)

	// Create temp directory
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tempDir)
	require.NoError(t, err)
	defer os.Chdir(originalWd)

	app, err := NewWithClient(
		context.Background(),
		&GlobalOption{},
		aws.Config{},
		mockCtrlClient,
		mockClient,
		mockECRClient,
		mockSTSClient,
	)
	require.NoError(t, err)

	opt := &InitOption{
		AgentRuntimeName: "test-runtime",
		Format:           "jsonnet",
		ForceOverwrite:   true,
	}

	err = app.Init(context.Background(), opt)
	require.NoError(t, err)

	// Verify file was created
	filename := filepath.Join(tempDir, "agent_runtime.jsonnet")
	require.FileExists(t, filename)

	// Verify file contents (Jsonnet format)
	content, err := os.ReadFile(filename)
	require.NoError(t, err)

	contentStr := string(content)
	require.Contains(t, contentStr, "agentRuntimeName:")
	require.Contains(t, contentStr, "test-runtime")
	require.Contains(t, contentStr, "roleArn:")
}

func TestInit_WithQualifier(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	qualifier := "2"

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
					AgentRuntimeName: aws.String("test-runtime"),
					AgentRuntimeArn:  aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/test-runtime-id"),
				},
			},
			NextToken: nil,
		}, nil)

	// Mock GetAgentRuntime response with specific version
	mockCtrlClient.EXPECT().
		GetAgentRuntime(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeOutput{
			AgentRuntimeId:      aws.String("test-runtime-id"),
			AgentRuntimeName:    aws.String("test-runtime"),
			AgentRuntimeArn:     aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/test-runtime-id"),
			AgentRuntimeVersion: aws.String("2"),
			RoleArn:             aws.String("arn:aws:iam::123456789012:role/test-role"),
			AgentRuntimeArtifact: &types.AgentRuntimeArtifactMemberContainerConfiguration{
				Value: types.ContainerConfiguration{
					ContainerUri: aws.String("123456789012.dkr.ecr.us-west-2.amazonaws.com/test:v2"),
				},
			},
		}, nil)

	// Create temp directory
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tempDir)
	require.NoError(t, err)
	defer os.Chdir(originalWd)

	app, err := NewWithClient(
		context.Background(),
		&GlobalOption{},
		aws.Config{},
		mockCtrlClient,
		mockClient,
		mockECRClient,
		mockSTSClient,
	)
	require.NoError(t, err)

	opt := &InitOption{
		AgentRuntimeName: "test-runtime",
		Qualifier:        &qualifier,
		Format:           "json",
		ForceOverwrite:   true,
	}

	err = app.Init(context.Background(), opt)
	require.NoError(t, err)

	// Verify file was created
	filename := filepath.Join(tempDir, "agent_runtime.json")
	require.FileExists(t, filename)
}
