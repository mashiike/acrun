package acrun

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcore"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestInvoke_WithPayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	mockClient := NewMockBedrockAgentCoreClient(ctrl)
	mockECRClient := NewMockECRClient(ctrl)
	mockSTSClient := NewMockSTSClient(ctrl)

	// Mock ListAgentRuntimes response (called by GetAgentRuntimeARNByName)
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

	// Mock InvokeAgentRuntime response
	responseBody := `{"result": "success", "message": "Hello from agent"}`
	mockClient.EXPECT().
		InvokeAgentRuntime(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, params *bedrockagentcore.InvokeAgentRuntimeInput, optFns ...func(*bedrockagentcore.Options)) (*bedrockagentcore.InvokeAgentRuntimeOutput, error) {
			// Verify input parameters
			require.Equal(t, "arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/test-runtime-id", *params.AgentRuntimeArn)
			require.Equal(t, `{"query": "test"}`, string(params.Payload))
			require.Equal(t, "application/json", *params.ContentType)
			require.Equal(t, "application/json", *params.Accept)

			return &bedrockagentcore.InvokeAgentRuntimeOutput{
				StatusCode:  aws.Int32(200),
				ContentType: aws.String("application/json"),
				Response:    io.NopCloser(bytes.NewBufferString(responseBody)),
			}, nil
		})

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

	payload := `{"query": "test"}`
	opt := &InvokeOption{
		Payload: &payload,
	}

	err = app.Invoke(context.Background(), opt)
	require.NoError(t, err)

	// Verify response body was written to stdout
	output := stdout.String()
	require.Equal(t, responseBody, output)
}

func TestInvoke_WithHeaders(t *testing.T) {
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

	// Mock InvokeAgentRuntime response with tracing headers
	responseBody := `{"result": "success"}`
	traceID := "test-trace-id"
	mcpSessionID := "test-mcp-session-id"
	runtimeSessionID := "test-runtime-session-id"

	mockClient.EXPECT().
		InvokeAgentRuntime(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, params *bedrockagentcore.InvokeAgentRuntimeInput, optFns ...func(*bedrockagentcore.Options)) (*bedrockagentcore.InvokeAgentRuntimeOutput, error) {
			// Verify tracing headers were passed
			require.Equal(t, traceID, *params.TraceId)
			require.Equal(t, mcpSessionID, *params.McpSessionId)
			require.Equal(t, runtimeSessionID, *params.RuntimeSessionId)

			return &bedrockagentcore.InvokeAgentRuntimeOutput{
				StatusCode:       aws.Int32(200),
				ContentType:      aws.String("application/json"),
				Response:         io.NopCloser(bytes.NewBufferString(responseBody)),
				TraceId:          aws.String(traceID),
				McpSessionId:     aws.String(mcpSessionID),
				RuntimeSessionId: aws.String(runtimeSessionID),
			}, nil
		})

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

	payload := `{"query": "test"}`
	opt := &InvokeOption{
		Payload:          &payload,
		TraceID:          &traceID,
		MCPSessionID:     &mcpSessionID,
		RuntimeSessionID: &runtimeSessionID,
	}

	err = app.Invoke(context.Background(), opt)
	require.NoError(t, err)

	// Verify response body
	output := stdout.String()
	require.Equal(t, responseBody, output)
}

func TestInvoke_AutoContentType(t *testing.T) {
	cases := []struct {
		Name       string
		Payload    string
		ExpectedCT string
	}{
		{
			Name:       "JSON payload",
			Payload:    `{"query": "test"}`,
			ExpectedCT: "application/json",
		},
		{
			Name:       "Plain text payload",
			Payload:    "plain text query",
			ExpectedCT: "text/plain",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
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

			// Mock InvokeAgentRuntime response
			responseBody := `{"result": "success"}`
			mockClient.EXPECT().
				InvokeAgentRuntime(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, params *bedrockagentcore.InvokeAgentRuntimeInput, optFns ...func(*bedrockagentcore.Options)) (*bedrockagentcore.InvokeAgentRuntimeOutput, error) {
					// Verify auto-detected content type
					require.Equal(t, tc.ExpectedCT, *params.ContentType)

					return &bedrockagentcore.InvokeAgentRuntimeOutput{
						StatusCode:  aws.Int32(200),
						ContentType: aws.String("application/json"),
						Response:    io.NopCloser(bytes.NewBufferString(responseBody)),
					}, nil
				})

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

			opt := &InvokeOption{
				Payload: &tc.Payload,
				// ContentType not specified - should be auto-detected
			}

			err = app.Invoke(context.Background(), opt)
			require.NoError(t, err)
		})
	}
}
