package acrun

import (
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
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
		{
			Name:     "with_allowed_ports_and_filesystem_configurations",
			File:     "testdata/agent_runtime_with_allowed_ports.json",
			Expected: agentRuntimeWithAllowedPorts(),
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

func agentRuntimeWithAllowedPorts() *AgentRuntime {
	return &AgentRuntime{
		AgentRuntimeName: aws.String("hosted_agent_with_ports"),
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
			ServerProtocol: types.ServerProtocolHttp,
			Path:           aws.String("/invoke"),
			Port:           aws.Int32(9000),
		},
		AllowedPorts: []types.AllowedPort{
			&types.AllowedPortMemberPort{Value: 9001},
			&types.AllowedPortMemberPortRange{Value: types.PortRange{From: aws.Int32(9100), To: aws.Int32(9110)}},
		},
		FilesystemConfigurations: []types.FilesystemConfiguration{
			&types.FilesystemConfigurationMemberSessionStorage{
				Value: types.SessionStorageConfiguration{MountPath: aws.String("/mnt/session")},
			},
			&types.FilesystemConfigurationMemberEfsAccessPoint{
				Value: types.EfsAccessPointConfiguration{
					AccessPointArn: aws.String("arn:aws:elasticfilesystem:us-west-2:123456789012:access-point/fsap-0123456789abcdef0"),
					MountPath:      aws.String("/mnt/efs"),
				},
			},
		},
	}
}

func TestAgentRuntimeFromResponseWithSliceUnions(t *testing.T) {
	expected := agentRuntimeWithAllowedPorts()
	out := &bedrockagentcorecontrol.GetAgentRuntimeOutput{
		AgentRuntimeId:           aws.String("hosted_agent_with_ports-abcdef"),
		AgentRuntimeName:         expected.AgentRuntimeName,
		RoleArn:                  expected.RoleArn,
		AgentRuntimeArtifact:     expected.AgentRuntimeArtifact,
		NetworkConfiguration:     expected.NetworkConfiguration,
		ProtocolConfiguration:    expected.ProtocolConfiguration,
		AllowedPorts:             expected.AllowedPorts,
		FilesystemConfigurations: expected.FilesystemConfigurations,
	}

	got, err := newAgentRuntimeFromResponse(out)
	require.NoError(t, err)
	require.EqualValues(t, expected, got)

	input, err := newUpdateAgentRuntimeInput(out, got)
	require.NoError(t, err)
	require.Equal(t, "hosted_agent_with_ports-abcdef", aws.ToString(input.AgentRuntimeId))
	require.EqualValues(t, expected.AllowedPorts, input.AllowedPorts)
	require.EqualValues(t, expected.FilesystemConfigurations, input.FilesystemConfigurations)
}

func TestConvertSliceUnion(t *testing.T) {
	cases := []struct {
		Name     string
		Input    any
		Strict   bool
		Expected []types.AllowedPort
		ErrMsg   string
	}{
		{
			Name:     "null keeps nil",
			Input:    nil,
			Expected: nil,
		},
		{
			Name:     "empty list keeps empty non-nil",
			Input:    []any{},
			Expected: []types.AllowedPort{},
		},
		{
			Name:   "not a list",
			Input:  map[string]any{"port": 9001},
			ErrMsg: "json: cannot unmarshal",
		},
		{
			Name:   "element without variant",
			Input:  []any{map[string]any{"port": 9001}, map[string]any{}},
			ErrMsg: "index 1: no valid variant found for AllowedPort",
		},
		{
			Name: "element with two variants",
			Input: []any{
				map[string]any{"port": 9001, "portRange": map[string]any{"from": 9100, "to": 9110}},
			},
			ErrMsg: "index 0: AllowedPort must have exactly one variant, got 2",
		},
		{
			Name:   "unknown field in element is rejected when strict",
			Input:  []any{map[string]any{"portRange": map[string]any{"from": 9100, "to": 9110, "step": 1}}},
			Strict: true,
			ErrMsg: `index 0: json: unknown field "step"`,
		},
		{
			Name:  "unknown field in element is ignored when not strict",
			Input: []any{map[string]any{"portRange": map[string]any{"from": 9100, "to": 9110, "step": 1}}},
			Expected: []types.AllowedPort{
				&types.AllowedPortMemberPortRange{Value: types.PortRange{From: aws.Int32(9100), To: aws.Int32(9110)}},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			got, err := convertToAllowedPortList(tc.Input, tc.Strict)
			if tc.ErrMsg != "" {
				require.ErrorContains(t, err, tc.ErrMsg)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.Expected, got)
		})
	}
}

func TestConvertFromSliceUnionRejectsNilElement(t *testing.T) {
	_, err := convertFromAllowedPortList([]types.AllowedPort{&types.AllowedPortMemberPort{Value: 9001}, nil})
	require.ErrorContains(t, err, "index 1: nil element")
}

func agentRuntimeWithNestedUnions() *AgentRuntime {
	return &AgentRuntime{
		AgentRuntimeName: aws.String("hosted_agent_with_nested_unions"),
		RoleArn:          aws.String("arn:aws:iam::123456789012:role/service-role/DummyServiceRole"),
		AgentRuntimeArtifact: &types.AgentRuntimeArtifactMemberCodeConfiguration{
			Value: types.CodeConfiguration{
				Code: &types.CodeMemberS3{
					Value: types.S3Location{Bucket: aws.String("example-bucket"), Prefix: aws.String("agent/code.zip")},
				},
				EntryPoint: []string{"main.py"},
				Runtime:    types.AgentManagedRuntimeTypePython312,
			},
		},
		NetworkConfiguration: &types.NetworkConfiguration{
			NetworkMode: types.NetworkModePublic,
		},
		AuthorizerConfiguration: &types.AuthorizerConfigurationMemberCustomJWTAuthorizer{
			Value: types.CustomJWTAuthorizerConfiguration{
				DiscoveryUrl:    aws.String("https://example.com/.well-known/openid-configuration"),
				AllowedAudience: []string{"example_audience"},
				CustomClaims: []types.CustomClaimValidationType{
					{
						InboundTokenClaimName:      aws.String("department"),
						InboundTokenClaimValueType: types.InboundTokenClaimValueTypeString,
						AuthorizingClaimMatchValue: &types.AuthorizingClaimMatchValueType{
							ClaimMatchOperator: types.ClaimMatchOperatorTypeEquals,
							ClaimMatchValue:    &types.ClaimMatchValueTypeMemberMatchValueString{Value: "engineering"},
						},
					},
					{
						InboundTokenClaimName:      aws.String("groups"),
						InboundTokenClaimValueType: types.InboundTokenClaimValueTypeStringArray,
						AuthorizingClaimMatchValue: &types.AuthorizingClaimMatchValueType{
							ClaimMatchOperator: types.ClaimMatchOperatorTypeContainsAny,
							ClaimMatchValue:    &types.ClaimMatchValueTypeMemberMatchValueStringList{Value: []string{"admin", "operator"}},
						},
					},
				},
				PrivateEndpoint: &types.PrivateEndpointMemberSelfManagedLatticeResource{
					Value: &types.SelfManagedLatticeResourceMemberResourceConfigurationIdentifier{Value: "rcfg-0123456789abcdef0"},
				},
				PrivateEndpointOverrides: []types.PrivateEndpointOverride{
					{
						Domain: aws.String("auth.example.com"),
						PrivateEndpoint: &types.PrivateEndpointMemberSelfManagedLatticeResource{
							Value: &types.SelfManagedLatticeResourceMemberResourceConfigurationIdentifier{Value: "rcfg-0fedcba9876543210"},
						},
					},
				},
			},
		},
	}
}

func TestNestedUnions(t *testing.T) {
	expected := agentRuntimeWithNestedUnions()

	t.Run("file to definition", func(t *testing.T) {
		bs, err := os.ReadFile("testdata/agent_runtime_with_nested_unions.json")
		require.NoError(t, err)
		got, err := unmarshalAgentRuntime(bs, true)
		require.NoError(t, err)
		require.EqualValues(t, expected, got)
	})

	t.Run("definition round trip", func(t *testing.T) {
		bs, err := marshalAgentRuntime(expected, "  ")
		require.NoError(t, err)
		got, err := unmarshalAgentRuntime(bs, true)
		require.NoError(t, err)
		require.EqualValues(t, expected, got)
	})

	t.Run("remote to definition to update input", func(t *testing.T) {
		out := &bedrockagentcorecontrol.GetAgentRuntimeOutput{
			AgentRuntimeId:          aws.String("hosted_agent_with_nested_unions-abcdef"),
			AgentRuntimeName:        expected.AgentRuntimeName,
			RoleArn:                 expected.RoleArn,
			AgentRuntimeArtifact:    expected.AgentRuntimeArtifact,
			NetworkConfiguration:    expected.NetworkConfiguration,
			AuthorizerConfiguration: expected.AuthorizerConfiguration,
		}
		got, err := newAgentRuntimeFromResponse(out)
		require.NoError(t, err)
		require.EqualValues(t, expected, got)

		input, err := newUpdateAgentRuntimeInput(out, got)
		require.NoError(t, err)
		require.EqualValues(t, expected.AgentRuntimeArtifact, input.AgentRuntimeArtifact)
		require.EqualValues(t, expected.AuthorizerConfiguration, input.AuthorizerConfiguration)
	})

	t.Run("upper camel keys", func(t *testing.T) {
		src := `{"AgentRuntimeName":"a","RoleArn":"r",
 "AgentRuntimeArtifact":{"codeConfiguration":{"Code":{"s3":{"Bucket":"b","Prefix":"p.zip"}},"EntryPoint":["main.py"],"Runtime":"PYTHON_3_12"}}}`
		got, err := unmarshalAgentRuntime([]byte(src), true)
		require.NoError(t, err)
		require.EqualValues(t, &types.AgentRuntimeArtifactMemberCodeConfiguration{
			Value: types.CodeConfiguration{
				Code:       &types.CodeMemberS3{Value: types.S3Location{Bucket: aws.String("b"), Prefix: aws.String("p.zip")}},
				EntryPoint: []string{"main.py"},
				Runtime:    types.AgentManagedRuntimeTypePython312,
			},
		}, got.AgentRuntimeArtifact)
	})

	t.Run("unknown field deep inside is reported by name", func(t *testing.T) {
		src := `{"agentRuntimeName":"a","roleArn":"r","authorizerConfiguration":{"customJWTAuthorizer":{"discoveryUrl":"u",
 "customClaims":[{"inboundTokenClaimName":"x","authorizingClaimMatchValue":{"claimMatchOperator":"EQUALS","claimMatchValue":{"matchValueString":"v"},"bogus":1}}]}}}`
		_, err := unmarshalAgentRuntime([]byte(src), true)
		require.Error(t, err)
		require.Equal(t, "bogus", extractUnknownFieldKey(err))

		got, err := unmarshalAgentRuntime([]byte(src), false)
		require.NoError(t, err)
		jwt := customJWTAuthorizer(t, got.AuthorizerConfiguration)
		require.Equal(t, &types.ClaimMatchValueTypeMemberMatchValueString{Value: "v"}, jwt.CustomClaims[0].AuthorizingClaimMatchValue.ClaimMatchValue)
	})

	t.Run("unknown field next to a nested union is rejected when strict", func(t *testing.T) {
		src := `{"agentRuntimeName":"a","roleArn":"r",
 "agentRuntimeArtifact":{"codeConfiguration":{"code":{"s3":{"bucket":"b"}},"runtime":"PYTHON_3_12","handler":"x"}}}`
		_, err := unmarshalAgentRuntime([]byte(src), true)
		require.Error(t, err)
		require.Equal(t, "handler", extractUnknownFieldKey(err))
	})
}

func customJWTAuthorizer(t *testing.T, v types.AuthorizerConfiguration) types.CustomJWTAuthorizerConfiguration {
	t.Helper()
	require.IsType(t, &types.AuthorizerConfigurationMemberCustomJWTAuthorizer{}, v)
	return v.(*types.AuthorizerConfigurationMemberCustomJWTAuthorizer).Value
}

func TestMapKeysKeepCase(t *testing.T) {
	src := `{"agentRuntimeName":"a","roleArn":"r",
 "tags":{"env":"dev","Team":"platform","app.kubernetes.io/name":"agent","Cost.center":"x","cost.center":"y"},
 "environmentVariables":{"lower":"1","UPPER":"2","my.var":"3","MY.VAR2":"4"},
 "authorizerConfiguration":{"customJWTAuthorizer":{"discoveryUrl":"u","advertisedScopeMapping":{"read":"Read:All","Write":"w","Api.read":"r"}}}}`
	wantTags := map[string]string{"env": "dev", "Team": "platform", "app.kubernetes.io/name": "agent", "Cost.center": "x", "cost.center": "y"}
	wantEnv := map[string]string{"lower": "1", "UPPER": "2", "my.var": "3", "MY.VAR2": "4"}
	wantScopes := map[string]string{"read": "Read:All", "Write": "w", "Api.read": "r"}
	scopes := func(v types.AuthorizerConfiguration) map[string]string {
		return customJWTAuthorizer(t, v).AdvertisedScopeMapping
	}

	def, err := unmarshalAgentRuntime([]byte(src), true)
	require.NoError(t, err)
	require.Equal(t, wantTags, def.Tags)
	require.Equal(t, wantEnv, def.EnvironmentVariables)
	require.Equal(t, wantScopes, scopes(def.AuthorizerConfiguration))

	bs, err := marshalAgentRuntime(def, "")
	require.NoError(t, err)
	roundTrip, err := unmarshalAgentRuntime(bs, true)
	require.NoError(t, err)
	require.Equal(t, wantTags, roundTrip.Tags)
	require.Equal(t, wantEnv, roundTrip.EnvironmentVariables)
	require.Equal(t, wantScopes, scopes(roundTrip.AuthorizerConfiguration))

	out := &bedrockagentcorecontrol.GetAgentRuntimeOutput{
		AgentRuntimeId:          aws.String("a-abcdef"),
		AgentRuntimeName:        def.AgentRuntimeName,
		RoleArn:                 def.RoleArn,
		EnvironmentVariables:    def.EnvironmentVariables,
		AuthorizerConfiguration: def.AuthorizerConfiguration,
	}
	remote, err := newAgentRuntimeFromResponse(out)
	require.NoError(t, err)
	require.Equal(t, wantEnv, remote.EnvironmentVariables)
	require.Equal(t, wantScopes, scopes(remote.AuthorizerConfiguration))

	input, err := newUpdateAgentRuntimeInput(out, def)
	require.NoError(t, err)
	require.Equal(t, wantEnv, input.EnvironmentVariables)
	require.Equal(t, wantScopes, scopes(input.AuthorizerConfiguration))
}

func TestUnionMemberMapKeysKeepCase(t *testing.T) {
	src := `{"agentRuntimeName":"a","roleArn":"r","authorizerConfiguration":{"customJWTAuthorizer":{"discoveryUrl":"u",
 "privateEndpoint":{"managedVpcResource":{"vpcIdentifier":"vpc-1","tags":{"Team":"platform","env":"dev"}}}}}}`
	want := map[string]string{"Team": "platform", "env": "dev"}
	vpcTags := func(def *AgentRuntime) map[string]string {
		pe := customJWTAuthorizer(t, def.AuthorizerConfiguration).PrivateEndpoint
		require.IsType(t, &types.PrivateEndpointMemberManagedVpcResource{}, pe)
		return pe.(*types.PrivateEndpointMemberManagedVpcResource).Value.Tags
	}

	def, err := unmarshalAgentRuntime([]byte(src), true)
	require.NoError(t, err)
	require.Equal(t, want, vpcTags(def))

	bs, err := marshalAgentRuntime(def, "")
	require.NoError(t, err)
	roundTrip, err := unmarshalAgentRuntime(bs, true)
	require.NoError(t, err)
	require.Equal(t, want, vpcTags(roundTrip))
}

func TestNestedUnionEdgeCases(t *testing.T) {
	t.Run("omitted nested lists stay omitted", func(t *testing.T) {
		src := `{"agentRuntimeName":"a","roleArn":"r","authorizerConfiguration":{"customJWTAuthorizer":{"discoveryUrl":"u"}}}`
		def, err := unmarshalAgentRuntime([]byte(src), true)
		require.NoError(t, err)
		bs, err := marshalAgentRuntime(def, "")
		require.NoError(t, err)
		require.NotContains(t, string(bs), "customClaims")
		require.NotContains(t, string(bs), "privateEndpointOverrides")
		roundTrip, err := unmarshalAgentRuntime(bs, true)
		require.NoError(t, err)
		require.EqualValues(t, def, roundTrip)
	})

	t.Run("upper camel keys in nested structs", func(t *testing.T) {
		src := `{"agentRuntimeName":"a","roleArn":"r","authorizerConfiguration":{"customJWTAuthorizer":{"DiscoveryUrl":"u",
 "CustomClaims":[{"InboundTokenClaimName":"x","AuthorizingClaimMatchValue":{"ClaimMatchOperator":"EQUALS","ClaimMatchValue":{"matchValueString":"v"}}}],
 "PrivateEndpointOverrides":[{"Domain":"d","PrivateEndpoint":{"selfManagedLatticeResource":{"resourceConfigurationIdentifier":"rcfg-1"}}}]}}}`
		def, err := unmarshalAgentRuntime([]byte(src), true)
		require.NoError(t, err)
		jwt := customJWTAuthorizer(t, def.AuthorizerConfiguration)
		require.Equal(t, &types.ClaimMatchValueTypeMemberMatchValueString{Value: "v"}, jwt.CustomClaims[0].AuthorizingClaimMatchValue.ClaimMatchValue)
		require.Equal(t, &types.PrivateEndpointMemberSelfManagedLatticeResource{
			Value: &types.SelfManagedLatticeResourceMemberResourceConfigurationIdentifier{Value: "rcfg-1"},
		}, jwt.PrivateEndpointOverrides[0].PrivateEndpoint)
	})

	t.Run("null element in a struct list is rejected", func(t *testing.T) {
		src := `{"agentRuntimeName":"a","roleArn":"r","authorizerConfiguration":{"customJWTAuthorizer":{"discoveryUrl":"u","customClaims":[null]}}}`
		_, err := unmarshalAgentRuntime([]byte(src), true)
		require.ErrorContains(t, err, "customClaims: index 0: null element")
	})

	t.Run("top-level union with two variants is rejected", func(t *testing.T) {
		src := `{"agentRuntimeName":"a","roleArn":"r","agentRuntimeArtifact":{
 "containerConfiguration":{"containerUri":"x"},
 "codeConfiguration":{"code":{"s3":{"bucket":"b"}},"runtime":"PYTHON_3_12"}}}`
		_, err := unmarshalAgentRuntime([]byte(src), true)
		require.ErrorContains(t, err, "agentRuntimeArtifact: AgentRuntimeArtifact must have exactly one variant, got 2")
	})
}
