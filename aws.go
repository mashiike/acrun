package acrun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcore"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

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

func newAgentRuntimeFromResponse(out *bedrockagentcorecontrol.GetAgentRuntimeOutput) (*AgentRuntime, error) {
	bs, err := marshalJSON(out, func(opts *marshalJSONOptions) {
		opts.hooks = append(opts.hooks, func(path, key string, value any) (string, any, error) {
			if matchJSONKey(path, "$.agentRuntimeArtifact") {
				v, err := convertFromAgentRuntimeArtifact(out.AgentRuntimeArtifact)
				if err != nil {
					return "", nil, err
				}
				return key, v, nil
			}
			if matchJSONKey(path, "$.AuthorizerConfiguration") {
				v, err := convertFromAuthorizerConfiguration(out.AuthorizerConfiguration)
				if err != nil {
					return "", nil, err
				}
				return key, v, nil
			}
			if matchJSONKey(path, "$.requestHeaderConfiguration") {
				v, err := convertFromRequestHeaderConfiguration(out.RequestHeaderConfiguration)
				if err != nil {
					return "", nil, err
				}
				return key, v, nil
			}
			return key, value, nil
		})
		opts.ignoreLowerCamelPaths = append(opts.ignoreLowerCamelPaths,
			"$.environmentVariables.*",
		)
	})
	if err != nil {
		return nil, err
	}
	def, err := unmarshalAgentRuntime(bs, false)
	if err != nil {
		return nil, err
	}
	return def, nil
}

func newUpdateAgentRuntimeInput(out *bedrockagentcorecontrol.GetAgentRuntimeOutput, def *AgentRuntime) (*bedrockagentcorecontrol.UpdateAgentRuntimeInput, error) {
	var input bedrockagentcorecontrol.UpdateAgentRuntimeInput
	unmarshalOpts := func(opts *unmarshalJSONOptions) {
		opts.strict = false
		opts.hooks = append(opts.hooks, func(path, key string, value any) (string, any, error) {
			if matchJSONKey(path, "$.agentRuntimeArtifact") {
				v, err := convertToAgentRuntimeArtifact(value, false)
				if err != nil {
					return "", nil, err
				}
				input.AgentRuntimeArtifact = v
				return key, nil, nil
			}
			if matchJSONKey(path, "$.AuthorizerConfiguration") {
				v, err := convertToAgentAuthorizerConfiguration(value, false)
				if err != nil {
					return "", nil, err
				}
				input.AuthorizerConfiguration = v
				return key, nil, nil
			}
			if matchJSONKey(path, "$.requestHeaderConfiguration") {
				v, err := convertToRequestHeaderConfiguration(value, false)
				if err != nil {
					return "", nil, err
				}
				input.RequestHeaderConfiguration = v
				return key, nil, nil
			}
			return key, value, nil
		})
		opts.ignoreUpperCamelPaths = append(opts.ignoreUpperCamelPaths,
			"$.environmentVariables.*",
		)
	}
	// Override with the definition file
	bs, err := marshalAgentRuntime(def, "")
	if err != nil {
		return nil, err
	}
	if err := unmarshalJSON(bs, &input, unmarshalOpts); err != nil {
		return nil, err
	}
	input.AgentRuntimeId = out.AgentRuntimeId
	return &input, nil
}

func unmarshalAgentRuntime(bs []byte, strict bool) (*AgentRuntime, error) {
	var def AgentRuntime
	hook := func(path, key string, value any) (string, any, error) {
		if matchJSONKey(path, "$.agentRuntimeArtifact") {
			v, err := convertToAgentRuntimeArtifact(value, strict)
			if err != nil {
				return "", nil, err
			}
			def.AgentRuntimeArtifact = v
			return key, nil, nil
		}
		if matchJSONKey(path, "$.AuthorizerConfiguration") {
			v, err := convertToAgentAuthorizerConfiguration(value, strict)
			if err != nil {
				return "", nil, err
			}
			def.AuthorizerConfiguration = v
			return key, nil, nil
		}
		if matchJSONKey(path, "$.requestHeaderConfiguration") {
			v, err := convertToRequestHeaderConfiguration(value, strict)
			if err != nil {
				return "", nil, err
			}
			def.RequestHeaderConfiguration = v
			return key, nil, nil
		}
		return key, value, nil
	}
	if err := unmarshalJSON(bs, &def, func(opts *unmarshalJSONOptions) {
		opts.hooks = append(opts.hooks, hook)
		opts.strict = strict
		opts.ignoreUpperCamelPaths = append(opts.ignoreUpperCamelPaths,
			"$.environmentVariables.*",
		)
	}); err != nil {
		return nil, err
	}
	return &def, nil
}

func marshalAgentRuntime(v *AgentRuntime, indent string) ([]byte, error) {
	bs, err := marshalJSON(v, func(opts *marshalJSONOptions) {
		opts.hooks = append(opts.hooks, func(path, key string, value any) (string, any, error) {
			if matchJSONKey(path, "$.agentRuntimeArtifact") {
				v, err := convertFromAgentRuntimeArtifact(v.AgentRuntimeArtifact)
				if err != nil {
					return "", nil, err
				}
				return key, v, nil
			}
			if matchJSONKey(path, "$.AuthorizerConfiguration") {
				v, err := convertFromAuthorizerConfiguration(v.AuthorizerConfiguration)
				if err != nil {
					return "", nil, err
				}
				return key, v, nil
			}
			if matchJSONKey(path, "$.requestHeaderConfiguration") {
				v, err := convertFromRequestHeaderConfiguration(v.RequestHeaderConfiguration)
				if err != nil {
					return "", nil, err
				}
				return key, v, nil
			}
			return key, value, nil
		})
		opts.ignoreLowerCamelPaths = append(opts.ignoreLowerCamelPaths,
			"$.environmentVariables.*",
		)
	})
	if err != nil {
		return nil, err
	}
	if indent == "" {
		return bs, nil
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, bs, "", indent); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func convertToAgentRuntimeArtifact(v any, strict bool) (types.AgentRuntimeArtifact, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if strict {
		dec.DisallowUnknownFields()
	}
	var cc struct {
		ContainerConfiguration types.ContainerConfiguration `json:"containerConfiguration"`
	}
	err = dec.Decode(&cc)
	if err == nil && cc.ContainerConfiguration.ContainerUri != nil {
		return &types.AgentRuntimeArtifactMemberContainerConfiguration{
			Value: cc.ContainerConfiguration,
		}, nil
	}
	return nil, err
}

func convertFromAgentRuntimeArtifact(v types.AgentRuntimeArtifact) (any, error) {
	if v == nil {
		return nil, nil
	}
	switch v := v.(type) {
	case *types.AgentRuntimeArtifactMemberContainerConfiguration:
		bs, err := marshalJSON(v.Value)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"containerConfiguration": json.RawMessage(bs),
		}, nil
	default:
		return nil, fmt.Errorf("unknown AgentArtifact type: %T", v)
	}
}

func convertToAgentAuthorizerConfiguration(v any, strict bool) (types.AuthorizerConfiguration, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if strict {
		dec.DisallowUnknownFields()
	}
	var jwtAuthorizer struct {
		CustomJWTAuthorizer types.CustomJWTAuthorizerConfiguration `json:"customJWTAuthorizer"`
	}
	err = dec.Decode(&jwtAuthorizer)
	if err == nil && jwtAuthorizer.CustomJWTAuthorizer.DiscoveryUrl != nil {
		return &types.AuthorizerConfigurationMemberCustomJWTAuthorizer{
			Value: jwtAuthorizer.CustomJWTAuthorizer,
		}, nil
	}
	return nil, err
}

func convertFromAuthorizerConfiguration(v types.AuthorizerConfiguration) (any, error) {
	if v == nil {
		return nil, nil
	}
	switch v := v.(type) {
	case *types.AuthorizerConfigurationMemberCustomJWTAuthorizer:
		bs, err := marshalJSON(v.Value)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"customJWTAuthorizer": json.RawMessage(bs),
		}, nil
	default:
		return nil, fmt.Errorf("unknown AuthorizerConfiguration type: %T", v)
	}
}

func convertToRequestHeaderConfiguration(v any, strict bool) (types.RequestHeaderConfiguration, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if strict {
		dec.DisallowUnknownFields()
	}
	var rhc struct {
		AllowList []string `json:"allowList"`
	}
	err = dec.Decode(&rhc)
	if err != nil {
		return nil, err
	}
	return &types.RequestHeaderConfigurationMemberRequestHeaderAllowlist{
		Value: rhc.AllowList,
	}, nil
}

func convertFromRequestHeaderConfiguration(v types.RequestHeaderConfiguration) (any, error) {
	if v == nil {
		return nil, nil
	}
	switch v := v.(type) {
	case *types.RequestHeaderConfigurationMemberRequestHeaderAllowlist:
		return map[string]any{
			"allowList": v.Value,
		}, nil
	default:
		return nil, fmt.Errorf("unknown RequestHeaderConfiguration type: %T", v)
	}
}
