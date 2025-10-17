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
