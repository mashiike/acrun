package acrun

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"unicode"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/fujiwara/ssm-lookup/ssm"
	"github.com/fujiwara/tfstate-lookup/tfstate"
	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/formatter"
)

func MakeVM(ctx context.Context, stsClient STSClient, ecrClient ECRClient, awsCfg aws.Config, globalOpts *GlobalOption) *jsonnet.VM {
	vm := jsonnet.MakeVM()
	for _, f := range defaultJsonnetNativeFuncs(ctx, stsClient, ecrClient, awsCfg) {
		vm.NativeFunction(f)
	}

	// Add tfstate native function if tfstate path is provided
	if globalOpts.TFState != "" {
		state, err := tfstate.ReadURL(ctx, globalOpts.TFState)
		if err != nil {
			slog.WarnContext(ctx, "Failed to read tfstate, tfstate() function will not be available", "path", globalOpts.TFState, "error", err)
		} else {
			for _, f := range state.JsonnetNativeFuncs(ctx) {
				vm.NativeFunction(f)
			}
			slog.DebugContext(ctx, "Loaded tfstate", "path", globalOpts.TFState)
		}
	}

	// Set external variables
	for k, v := range globalOpts.ExtStr {
		vm.ExtVar(k, v)
	}

	// external code
	for k, v := range globalOpts.ExtCode {
		vm.ExtCode(k, v)
	}

	return vm
}

func jsonToJsonnet(src []byte, filepath string) ([]byte, error) {
	s, err := formatter.Format(filepath, string(src), formatter.DefaultOptions())
	if err != nil {
		return nil, fmt.Errorf("failed to format jsonnet: %w", err)
	}
	return []byte(s), nil
}

func defaultJsonnetNativeFuncs(ctx context.Context, stsClient STSClient, ecrClient ECRClient, awsCfg aws.Config) []*jsonnet.NativeFunction {
	nativeFunctions := []*jsonnet.NativeFunction{
		{
			Name:   "env",
			Params: []ast.Identifier{"name", "default"},
			Func: func(args []any) (any, error) {
				key, ok := args[0].(string)
				if !ok {
					return nil, fmt.Errorf("env: name must be a string")
				}
				if v := os.Getenv(key); v != "" {
					return v, nil
				}
				return args[1], nil
			},
		},
		{
			Name:   "mustEnv",
			Params: []ast.Identifier{"name"},
			Func: func(args []any) (any, error) {
				key, ok := args[0].(string)
				if !ok {
					return nil, fmt.Errorf("mustEnv: name must be a string")
				}
				if v, ok := os.LookupEnv(key); ok {
					return v, nil
				}
				return nil, fmt.Errorf("mustEnv: %s is not set", key)
			},
		},
		{
			Name:   "callerIdentity",
			Params: []ast.Identifier{},
			Func: func(args []any) (any, error) {
				if stsClient == nil {
					return nil, fmt.Errorf("callerIdentity: STS client is not available")
				}
				output, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
				if err != nil {
					return nil, fmt.Errorf("callerIdentity: failed to get caller identity: %w", err)
				}
				return map[string]any{
					"account": aws.ToString(output.Account),
					"arn":     aws.ToString(output.Arn),
					"userId":  aws.ToString(output.UserId),
				}, nil
			},
		},
		{
			Name:   "ecrImageUri",
			Params: []ast.Identifier{"repositoryName", "imageTag"},
			Func: func(args []any) (any, error) {
				if ecrClient == nil {
					return nil, fmt.Errorf("ecrImageUri: ECR client is not available")
				}

				repositoryName, ok := args[0].(string)
				if !ok {
					return nil, fmt.Errorf("ecrImageUri: repositoryName must be a string")
				}
				imageTag, ok := args[1].(string)
				if !ok {
					return nil, fmt.Errorf("ecrImageUri: imageTag must be a string")
				}

				// Verify repository exists and get registry info
				describeOutput, err := ecrClient.DescribeRepositories(ctx, &ecr.DescribeRepositoriesInput{
					RepositoryNames: []string{repositoryName},
				})
				if err != nil {
					return nil, fmt.Errorf("ecrImageUri: failed to describe repository: %w", err)
				}
				if len(describeOutput.Repositories) == 0 {
					return nil, fmt.Errorf("ecrImageUri: repository not found: %s", repositoryName)
				}

				repo := describeOutput.Repositories[0]

				// Construct ECR URI with the tag
				// repositoryURI is like "123456789012.dkr.ecr.us-west-2.amazonaws.com/repo-name"
				uri := fmt.Sprintf("%s:%s", aws.ToString(repo.RepositoryUri), imageTag)

				return uri, nil
			},
		},
	}
	cache := &sync.Map{}
	ssmlookup := ssm.New(awsCfg, cache)
	ssmlookupNFs := ssmlookup.JsonnetNativeFuncs(ctx)
	for i, f := range ssmlookupNFs {
		// Rename ssm_lookup to ssmLookup
		ssmlookupNFs[i].Name = ToLowerCamelCase(f.Name)
	}
	nativeFunctions = append(nativeFunctions, ssmlookupNFs...)
	return nativeFunctions
}

// ToLowerCamelCase converts a snake_case string to lowerCamelCase.
func ToLowerCamelCase(s string) string {
	parts := strings.Split(s, "_")
	if len(parts) == 0 {
		return ""
	}

	// Lowercase the first part
	result := strings.ToLower(parts[0])

	// Capitalize the first letter of subsequent parts
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			r := []rune(parts[i])
			r[0] = unicode.ToUpper(r[0])
			result += string(r)
		}
	}
	return result
}
