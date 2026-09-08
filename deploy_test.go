package acrun

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strconv"
	"testing"
	"time"

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

	// Wait for runtime to be ready
	gomock.InOrder(
		mockCtrlClient.EXPECT().
			GetAgentRuntime(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&bedrockagentcorecontrol.GetAgentRuntimeOutput{
				AgentRuntimeId:      aws.String("new-runtime-id"),
				AgentRuntimeArn:     aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/new-runtime-id"),
				AgentRuntimeVersion: aws.String("1"),
				Status:              types.AgentRuntimeStatusCreating,
			}, nil).Times(1),
		mockCtrlClient.EXPECT().
			GetAgentRuntime(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&bedrockagentcorecontrol.GetAgentRuntimeOutput{
				AgentRuntimeId:      aws.String("new-runtime-id"),
				AgentRuntimeArn:     aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/new-runtime-id"),
				AgentRuntimeVersion: aws.String("1"),
				Status:              types.AgentRuntimeStatusReady,
			}, nil).Times(1),
	)
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
		DryRun:          false,
		EndpointName:    &endpointName,
		WaitDuration:    1 * time.Minute,
		PollingInterval: 15 * time.Nanosecond,
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

	// Wait for runtime to be ready
	gomock.InOrder(
		mockCtrlClient.EXPECT().
			GetAgentRuntime(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&bedrockagentcorecontrol.GetAgentRuntimeOutput{
				AgentRuntimeId:      aws.String("existing-runtime-id"),
				AgentRuntimeArn:     aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/existing-runtime-id"),
				AgentRuntimeVersion: aws.String("2"),
				Status:              types.AgentRuntimeStatusUpdating,
			}, nil).Times(1),
		mockCtrlClient.EXPECT().
			GetAgentRuntime(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&bedrockagentcorecontrol.GetAgentRuntimeOutput{
				AgentRuntimeId:      aws.String("existing-runtime-id"),
				AgentRuntimeArn:     aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/existing-runtime-id"),
				AgentRuntimeVersion: aws.String("2"),
				Status:              types.AgentRuntimeStatusReady,
			}, nil).Times(1),
	)

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
		DryRun:          false,
		EndpointName:    &endpointName,
		WaitDuration:    1 * time.Minute,
		PollingInterval: 15 * time.Nanosecond,
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

func expectDeployUpdateForKeepVersions(mockCtrlClient *MockBedrockAgentCoreControlClient) {
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
		}, nil).AnyTimes()

	endpoints := map[string]struct{ live, target string }{
		"test-endpoint":     {live: "3", target: "11"},
		"production":        {live: "7", target: "7"},
		DefaultEndpointName: {live: "1", target: "1"},
	}
	mockCtrlClient.EXPECT().
		GetAgentRuntimeEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, in *bedrockagentcorecontrol.GetAgentRuntimeEndpointInput, _ ...func(*bedrockagentcorecontrol.Options)) (*bedrockagentcorecontrol.GetAgentRuntimeEndpointOutput, error) {
			e, ok := endpoints[aws.ToString(in.EndpointName)]
			if !ok {
				return nil, &types.ResourceNotFoundException{}
			}
			return &bedrockagentcorecontrol.GetAgentRuntimeEndpointOutput{
				LiveVersion:   aws.String(e.live),
				TargetVersion: aws.String(e.target),
				Description:   aws.String("managed by acrun"),
			}, nil
		}).AnyTimes()

	mockCtrlClient.EXPECT().
		GetAgentRuntime(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeOutput{
			AgentRuntimeId:      aws.String("existing-runtime-id"),
			AgentRuntimeArn:     aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/existing-runtime-id"),
			AgentRuntimeVersion: aws.String("12"),
			Status:              types.AgentRuntimeStatusReady,
		}, nil).AnyTimes()

	mockCtrlClient.EXPECT().
		UpdateAgentRuntime(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.UpdateAgentRuntimeOutput{
			AgentRuntimeId:      aws.String("existing-runtime-id"),
			AgentRuntimeVersion: aws.String("12"),
			AgentRuntimeArn:     aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/existing-runtime-id"),
		}, nil).AnyTimes()

	mockCtrlClient.EXPECT().
		UpdateAgentRuntimeEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.UpdateAgentRuntimeEndpointOutput{
			AgentRuntimeEndpointArn: aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:endpoint/test"),
		}, nil).AnyTimes()

	mockCtrlClient.EXPECT().
		ListAgentRuntimeEndpoints(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.ListAgentRuntimeEndpointsOutput{
			RuntimeEndpoints: []types.AgentRuntimeEndpoint{
				{Name: aws.String("test-endpoint")},
				{Name: aws.String("production")},
				{Name: aws.String(DefaultEndpointName)},
			},
		}, nil).Times(1)
}

func readyVersions(versions ...string) []types.AgentRuntime {
	out := make([]types.AgentRuntime, 0, len(versions))
	for _, v := range versions {
		out = append(out, types.AgentRuntime{
			AgentRuntimeVersion: aws.String(v),
			Status:              types.AgentRuntimeStatusReady,
		})
	}
	return out
}

func expectAgentRuntimeVersionPages(mockCtrlClient *MockBedrockAgentCoreControlClient, pages ...[]types.AgentRuntime) {
	calls := make([]any, 0, len(pages))
	for i, page := range pages {
		out := &bedrockagentcorecontrol.ListAgentRuntimeVersionsOutput{AgentRuntimes: page}
		if i < len(pages)-1 {
			out.NextToken = aws.String("page" + strconv.Itoa(i+2))
		}
		calls = append(calls, mockCtrlClient.EXPECT().
			ListAgentRuntimeVersions(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(out, nil).Times(1))
	}
	gomock.InOrder(calls...)
}

func deployedVersionPages() [][]types.AgentRuntime {
	page2 := readyVersions("10", "1")
	page2 = append(page2, types.AgentRuntime{
		AgentRuntimeVersion: aws.String("5"),
		Status:              types.AgentRuntimeStatusDeleting,
	})
	page2 = append(page2, readyVersions("3", "9", "11")...)
	return [][]types.AgentRuntime{readyVersions("2", "12", "7"), page2}
}

func notYetDeployedVersionPages() [][]types.AgentRuntime {
	pages := deployedVersionPages()
	pages[0] = readyVersions("2", "7")
	return pages
}

func newKeepVersionsApp(t *testing.T, ctrl *gomock.Controller, mockCtrlClient *MockBedrockAgentCoreControlClient, verbose bool) (*App, *bytes.Buffer) {
	t.Helper()
	app, err := NewWithClient(
		context.Background(),
		&GlobalOption{AgentRuntime: "testdata/agent_runtime.json", Verbose: verbose},
		aws.Config{},
		mockCtrlClient,
		NewMockBedrockAgentCoreClient(ctrl),
		NewMockECRClient(ctrl),
		NewMockSTSClient(ctrl),
	)
	require.NoError(t, err)
	var stdout, stderr bytes.Buffer
	app.SetOutput(&stdout, &stderr)
	return app, &stderr
}

func keepVersionsDeployOption(keepVersions int, dryRun bool) *DeployOption {
	endpointName := "test-endpoint"
	return &DeployOption{
		DryRun:          dryRun,
		EndpointName:    &endpointName,
		KeepVersions:    keepVersions,
		WaitDuration:    1 * time.Minute,
		PollingInterval: 15 * time.Nanosecond,
	}
}

func recordDeletedVersions(mockCtrlClient *MockBedrockAgentCoreControlClient, fail func(version string) error) *[]string {
	var recorded []string
	mockCtrlClient.EXPECT().
		DeleteAgentRuntime(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, in *bedrockagentcorecontrol.DeleteAgentRuntimeInput, _ ...func(*bedrockagentcorecontrol.Options)) (*bedrockagentcorecontrol.DeleteAgentRuntimeOutput, error) {
			v := aws.ToString(in.AgentRuntimeVersion)
			recorded = append(recorded, v)
			if fail != nil {
				if err := fail(v); err != nil {
					return nil, err
				}
			}
			return &bedrockagentcorecontrol.DeleteAgentRuntimeOutput{
				AgentRuntimeId:      in.AgentRuntimeId,
				AgentRuntimeVersion: in.AgentRuntimeVersion,
				Status:              types.AgentRuntimeStatusDeleting,
			}, nil
		}).AnyTimes()
	return &recorded
}

func TestDeploy_KeepVersions(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	expectDeployUpdateForKeepVersions(mockCtrlClient)
	expectAgentRuntimeVersionPages(mockCtrlClient, deployedVersionPages()...)
	deleted := recordDeletedVersions(mockCtrlClient, nil)

	app, _ := newKeepVersionsApp(t, ctrl, mockCtrlClient, false)
	err := app.Deploy(context.Background(), keepVersionsDeployOption(2, false))
	require.NoError(t, err)
	require.Equal(t, []string{"10", "9", "2"}, *deleted)
}

func TestDeploy_KeepVersionsDryRunPredictsActualDeletion(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	expectDeployUpdateForKeepVersions(mockCtrlClient)
	expectAgentRuntimeVersionPages(mockCtrlClient, notYetDeployedVersionPages()...)

	app, stderr := newKeepVersionsApp(t, ctrl, mockCtrlClient, true)
	err := app.Deploy(context.Background(), keepVersionsDeployOption(2, true))
	require.NoError(t, err)

	re := regexp.MustCompile(`DeleteAgentRuntimeInput:\n\{[^}]*"AgentRuntimeVersion": "(\d+)"`)
	var planned []string
	for _, m := range re.FindAllStringSubmatch(stderr.String(), -1) {
		planned = append(planned, m[1])
	}
	require.Equal(t, []string{"10", "9", "2"}, planned)
}

func TestDeploy_KeepVersionsAlreadyDeletedVersionIsIgnored(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	expectDeployUpdateForKeepVersions(mockCtrlClient)
	expectAgentRuntimeVersionPages(mockCtrlClient, deployedVersionPages()...)
	attempted := recordDeletedVersions(mockCtrlClient, func(version string) error {
		if version == "9" {
			return &types.ResourceNotFoundException{}
		}
		return nil
	})

	app, _ := newKeepVersionsApp(t, ctrl, mockCtrlClient, false)
	err := app.Deploy(context.Background(), keepVersionsDeployOption(2, false))
	require.NoError(t, err)
	require.Equal(t, []string{"10", "9", "2"}, *attempted)
}

func TestDeploy_KeepVersionsSkippedWhenVersionUnparsable(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	expectDeployUpdateForKeepVersions(mockCtrlClient)
	expectAgentRuntimeVersionPages(mockCtrlClient, []types.AgentRuntime{
		{AgentRuntimeVersion: aws.String("LATEST"), Status: types.AgentRuntimeStatusReady},
	})
	attempted := recordDeletedVersions(mockCtrlClient, nil)

	app, _ := newKeepVersionsApp(t, ctrl, mockCtrlClient, false)
	err := app.Deploy(context.Background(), keepVersionsDeployOption(2, false))
	require.NoError(t, err)
	require.Empty(t, *attempted)
}

func TestDeploy_KeepVersionsSkippedOnCreate(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	mockCtrlClient.EXPECT().
		ListAgentRuntimes(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.ListAgentRuntimesOutput{}, nil).AnyTimes()
	mockCtrlClient.EXPECT().
		GetAgentRuntimeEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, &types.ResourceNotFoundException{}).AnyTimes()

	app, _ := newKeepVersionsApp(t, ctrl, mockCtrlClient, false)
	err := app.Deploy(context.Background(), keepVersionsDeployOption(2, true))
	require.NoError(t, err)
}

func TestDeploy_KeepVersionsSkippedWhenEndpointLookupFails(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
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
		}, nil).AnyTimes()
	mockCtrlClient.EXPECT().
		GetAgentRuntimeEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, in *bedrockagentcorecontrol.GetAgentRuntimeEndpointInput, _ ...func(*bedrockagentcorecontrol.Options)) (*bedrockagentcorecontrol.GetAgentRuntimeEndpointOutput, error) {
			if aws.ToString(in.EndpointName) == "production" {
				return nil, &types.AccessDeniedException{}
			}
			return &bedrockagentcorecontrol.GetAgentRuntimeEndpointOutput{
				LiveVersion:   aws.String("3"),
				TargetVersion: aws.String("11"),
			}, nil
		}).AnyTimes()
	mockCtrlClient.EXPECT().
		GetAgentRuntime(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.GetAgentRuntimeOutput{
			AgentRuntimeId:      aws.String("existing-runtime-id"),
			AgentRuntimeVersion: aws.String("12"),
			Status:              types.AgentRuntimeStatusReady,
		}, nil).AnyTimes()
	mockCtrlClient.EXPECT().
		UpdateAgentRuntime(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.UpdateAgentRuntimeOutput{
			AgentRuntimeId:      aws.String("existing-runtime-id"),
			AgentRuntimeVersion: aws.String("12"),
		}, nil).AnyTimes()
	mockCtrlClient.EXPECT().
		UpdateAgentRuntimeEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.UpdateAgentRuntimeEndpointOutput{}, nil).AnyTimes()
	mockCtrlClient.EXPECT().
		ListAgentRuntimeEndpoints(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&bedrockagentcorecontrol.ListAgentRuntimeEndpointsOutput{
			RuntimeEndpoints: []types.AgentRuntimeEndpoint{
				{Name: aws.String("test-endpoint")},
				{Name: aws.String("production")},
			},
		}, nil).Times(1)

	app, _ := newKeepVersionsApp(t, ctrl, mockCtrlClient, false)
	err := app.Deploy(context.Background(), keepVersionsDeployOption(2, false))
	require.NoError(t, err)
}

func TestDeploy_KeepVersionsContinuesAfterDeleteFailure(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCtrlClient := NewMockBedrockAgentCoreControlClient(ctrl)
	expectDeployUpdateForKeepVersions(mockCtrlClient)
	expectAgentRuntimeVersionPages(mockCtrlClient, deployedVersionPages()...)
	attempted := recordDeletedVersions(mockCtrlClient, func(version string) error {
		if version == "9" {
			return errors.New("throttled")
		}
		return nil
	})

	app, _ := newKeepVersionsApp(t, ctrl, mockCtrlClient, false)
	err := app.Deploy(context.Background(), keepVersionsDeployOption(2, false))
	require.NoError(t, err)
	require.Equal(t, []string{"10", "9", "2"}, *attempted)
}
