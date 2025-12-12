package acrun

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcore"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

//go:generate go tool codegen
//go:generate go tool mockgen -source=aws.go -destination=./mock_test.go -package=acrun
type BedrockAgentCoreControlClient interface {
	ListAgentRuntimes(ctx context.Context, params *bedrockagentcorecontrol.ListAgentRuntimesInput, optFns ...func(*bedrockagentcorecontrol.Options)) (*bedrockagentcorecontrol.ListAgentRuntimesOutput, error)
	ListAgentRuntimeVersions(ctx context.Context, params *bedrockagentcorecontrol.ListAgentRuntimeVersionsInput, optFns ...func(*bedrockagentcorecontrol.Options)) (*bedrockagentcorecontrol.ListAgentRuntimeVersionsOutput, error)
	ListAgentRuntimeEndpoints(ctx context.Context, params *bedrockagentcorecontrol.ListAgentRuntimeEndpointsInput, optFns ...func(*bedrockagentcorecontrol.Options)) (*bedrockagentcorecontrol.ListAgentRuntimeEndpointsOutput, error)
	GetAgentRuntime(ctx context.Context, params *bedrockagentcorecontrol.GetAgentRuntimeInput, optFns ...func(*bedrockagentcorecontrol.Options)) (*bedrockagentcorecontrol.GetAgentRuntimeOutput, error)
	GetAgentRuntimeEndpoint(ctx context.Context, params *bedrockagentcorecontrol.GetAgentRuntimeEndpointInput, optFns ...func(*bedrockagentcorecontrol.Options)) (*bedrockagentcorecontrol.GetAgentRuntimeEndpointOutput, error)
	CreateAgentRuntime(ctx context.Context, params *bedrockagentcorecontrol.CreateAgentRuntimeInput, optFns ...func(*bedrockagentcorecontrol.Options)) (*bedrockagentcorecontrol.CreateAgentRuntimeOutput, error)
	CreateAgentRuntimeEndpoint(ctx context.Context, params *bedrockagentcorecontrol.CreateAgentRuntimeEndpointInput, optFns ...func(*bedrockagentcorecontrol.Options)) (*bedrockagentcorecontrol.CreateAgentRuntimeEndpointOutput, error)
	UpdateAgentRuntime(ctx context.Context, params *bedrockagentcorecontrol.UpdateAgentRuntimeInput, optFns ...func(*bedrockagentcorecontrol.Options)) (*bedrockagentcorecontrol.UpdateAgentRuntimeOutput, error)
	UpdateAgentRuntimeEndpoint(ctx context.Context, params *bedrockagentcorecontrol.UpdateAgentRuntimeEndpointInput, optFns ...func(*bedrockagentcorecontrol.Options)) (*bedrockagentcorecontrol.UpdateAgentRuntimeEndpointOutput, error)
	DeleteAgentRuntime(ctx context.Context, params *bedrockagentcorecontrol.DeleteAgentRuntimeInput, optFns ...func(*bedrockagentcorecontrol.Options)) (*bedrockagentcorecontrol.DeleteAgentRuntimeOutput, error)
	DeleteAgentRuntimeEndpoint(ctx context.Context, params *bedrockagentcorecontrol.DeleteAgentRuntimeEndpointInput, optFns ...func(*bedrockagentcorecontrol.Options)) (*bedrockagentcorecontrol.DeleteAgentRuntimeEndpointOutput, error)
}

type BedrockAgentCoreClient interface {
	InvokeAgentRuntime(ctx context.Context, params *bedrockagentcore.InvokeAgentRuntimeInput, optFns ...func(*bedrockagentcore.Options)) (*bedrockagentcore.InvokeAgentRuntimeOutput, error)
}

type ECRClient interface {
	DescribeRepositories(ctx context.Context, params *ecr.DescribeRepositoriesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeRepositoriesOutput, error)
}

type STSClient interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

type AgentRuntime = bedrockagentcorecontrol.CreateAgentRuntimeInput
