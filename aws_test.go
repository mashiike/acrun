package acrun

import (
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalAgentRuntime(t *testing.T) {

	cases := []struct {
		Name      string
		File      string
		ShouldErr bool
		Expected  *AgentRuntime
	}{
		{
			Name: "valid",
			File: "testdata/agent_runtime.json",
			Expected: &AgentRuntime{
				AgentRuntimeName: aws.String("hosted_agent_dummy"),
				RoleArn:          aws.String("arn:aws:iam::123456789012:role/service-role/DummyServiceRole"),
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
			},
		},
		{
			Name: "with_request_header_configuration",
			File: "testdata/agent_runtime_with_request_header.json",
			Expected: &AgentRuntime{
				AgentRuntimeName: aws.String("hosted_agent_with_header"),
				RoleArn:          aws.String("arn:aws:iam::123456789012:role/service-role/DummyServiceRole"),
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
				RequestHeaderConfiguration: &types.RequestHeaderConfigurationMemberRequestHeaderAllowlist{
					Value: []string{
						"X-Custom-Header",
						"X-Request-ID",
						"Authorization",
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			bs, err := os.ReadFile(tc.File)
			require.NoError(t, err)
			got, err := unmarshalAgentRuntime(bs, true)
			if tc.ShouldErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.EqualValues(t, tc.Expected, got)
		})
	}
}

func TestConvertRequestHeaderConfiguration(t *testing.T) {
	cases := []struct {
		Name      string
		Input     any
		Expected  types.RequestHeaderConfiguration
		ShouldErr bool
	}{
		{
			Name: "valid_allowlist",
			Input: map[string]any{
				"allowList": []any{
					"X-Custom-Header",
					"X-Request-ID",
					"Authorization",
				},
			},
			Expected: &types.RequestHeaderConfigurationMemberRequestHeaderAllowlist{
				Value: []string{
					"X-Custom-Header",
					"X-Request-ID",
					"Authorization",
				},
			},
			ShouldErr: false,
		},
		{
			Name: "empty_allowlist",
			Input: map[string]any{
				"allowList": []any{},
			},
			Expected: &types.RequestHeaderConfigurationMemberRequestHeaderAllowlist{
				Value: []string{},
			},
			ShouldErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			// Test convertToRequestHeaderConfiguration
			got, err := convertToRequestHeaderConfiguration(tc.Input, true)
			if tc.ShouldErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.EqualValues(t, tc.Expected, got)

			// Test round-trip through convertFromRequestHeaderConfiguration
			converted, err := convertFromRequestHeaderConfiguration(got)
			require.NoError(t, err)

			// Convert back again
			roundTrip, err := convertToRequestHeaderConfiguration(converted, true)
			require.NoError(t, err)
			require.EqualValues(t, tc.Expected, roundTrip)
		})
	}
}

func TestMarshalAgentRuntimeWithRequestHeaderConfiguration(t *testing.T) {
	runtime := &AgentRuntime{
		AgentRuntimeName: aws.String("test_runtime"),
		RoleArn:          aws.String("arn:aws:iam::123456789012:role/TestRole"),
		AgentRuntimeArtifact: &types.AgentRuntimeArtifactMemberContainerConfiguration{
			Value: types.ContainerConfiguration{
				ContainerUri: aws.String("123456789012.dkr.ecr.us-west-2.amazonaws.com/test:latest"),
			},
		},
		RequestHeaderConfiguration: &types.RequestHeaderConfigurationMemberRequestHeaderAllowlist{
			Value: []string{
				"X-Custom-Header",
				"Authorization",
			},
		},
	}

	// Marshal
	bs, err := marshalAgentRuntime(runtime, "  ")
	require.NoError(t, err)

	// Unmarshal and verify
	got, err := unmarshalAgentRuntime(bs, true)
	require.NoError(t, err)
	require.EqualValues(t, runtime, got)
}
