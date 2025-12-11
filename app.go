// Package acrun is a core application logic of acrun command line tool.
package acrun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/Songmu/prompter"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcore"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/google/go-jsonnet"
)

const (
	AppName             = "acrun"
	DefaultEndpointName = "DEFAULT"
)

var (
	DefaultAgentRuntimeFilenames = []string{
		"agent_runtime.json",
		"agent_runtime.jsonnet",
	}
	CurrentEndpointName = "current"
)

func fillEndpointName(name *string) string {
	if name == nil || *name == "" {
		return CurrentEndpointName
	}
	return *name
}

type App struct {
	agentRuntimeFilepath string
	ctrlClient           BedrockAgentCoreControlClient
	client               BedrockAgentCoreClient
	vm                   *jsonnet.VM

	cacheMu         sync.RWMutex
	cacheIDbyNames  map[string]string
	cacheARNbyNames map[string]string

	verbose bool
	stdout  io.Writer
	stderr  io.Writer
}

type GlobalOption struct {
	AgentRuntime string            `help:"Agent runtime file path"  json:"agent_runtime,omitempty"`
	TFState      string            `help:"Terraform state file URL (s3://... or local path)" env:"ACRUN_TFSTATE" json:"tfstate,omitempty"`
	ExtStr       map[string]string `help:"Set external string variable for Jsonnet VM" env:"ACRUN_EXTSTR" json:"ext_strs,omitempty"`
	ExtCode      map[string]string `help:"Set external code variable for Jsonnet VM" env:"ACRUN_EXTCODE" json:"ext_codes,omitempty"`
	Verbose      bool              `name:"verbose" short:"v" help:"enable verbose logging" default:"false" json:"verbose,omitempty"`
}

func New(ctx context.Context, opts *GlobalOption) (*App, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return NewWithClient(
		ctx,
		opts,
		awsCfg,
		bedrockagentcorecontrol.NewFromConfig(awsCfg),
		bedrockagentcore.NewFromConfig(awsCfg),
		ecr.NewFromConfig(awsCfg),
		sts.NewFromConfig(awsCfg),
	)
}

func NewWithClient(
	ctx context.Context,
	opts *GlobalOption,
	awsCfg aws.Config,
	ctrlClient BedrockAgentCoreControlClient,
	client BedrockAgentCoreClient,
	ecrClient ECRClient,
	stsClient STSClient,
) (*App, error) {
	if opts.AgentRuntime == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get current directory: %w", err)
		}
		for _, fn := range DefaultAgentRuntimeFilenames {
			path := filepath.Join(cwd, fn)
			if _, err := os.Stat(path); err == nil {
				slog.DebugContext(ctx, "Found agent runtime file", "file", path)
				opts.AgentRuntime = path
				break
			}
		}
	}

	return &App{
		agentRuntimeFilepath: opts.AgentRuntime,
		ctrlClient:           ctrlClient,
		client:               client,
		cacheIDbyNames:       make(map[string]string),
		cacheARNbyNames:      make(map[string]string),
		vm:                   MakeVM(ctx, stsClient, ecrClient, awsCfg, opts),
		stdout:               os.Stdout,
		stderr:               os.Stderr,
		verbose:              opts.Verbose,
	}, nil
}

var (
	ErrAgentRuntimeNotFound = errors.New("AgentRuntime not found")
)

func (app *App) SetOutput(stdout, stderr io.Writer) {
	app.stdout = stdout
	app.stderr = stderr
}

func (app *App) GetAgentRuntimeVersionByEndpointName(ctx context.Context, name string, endpointName string) (string, error) {
	id, err := app.GetAgentRuntimeIDByName(ctx, name)
	if err != nil {
		return "", fmt.Errorf("get agent runtime ID by name: %w", err)
	}
	v, err := app.ctrlClient.GetAgentRuntimeEndpoint(ctx, &bedrockagentcorecontrol.GetAgentRuntimeEndpointInput{
		AgentRuntimeId: aws.String(id),
		EndpointName:   aws.String(endpointName),
	})
	if err != nil {
		var nfe *types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			return "", fmt.Errorf("get agent runtime endpoint: %w", ErrAgentRuntimeNotFound)
		}
		var ade *types.AccessDeniedException
		if errors.As(err, &ade) {
			return "", fmt.Errorf("get agent runtime endpoint: %w", ErrAgentRuntimeNotFound)
		}
		return "", fmt.Errorf("GetAgentRuntimeEndpoint: %w", err)
	}
	if v.TargetVersion != nil {
		return aws.ToString(v.TargetVersion), nil
	}
	return aws.ToString(v.LiveVersion), nil
}

func (app *App) GetAgentRuntime(ctx context.Context, name *string, qualifier *string) (*bedrockagentcorecontrol.GetAgentRuntimeOutput, error) {
	id, err := app.GetAgentRuntimeIDByName(ctx, *name)
	if err != nil {
		return nil, fmt.Errorf("get agent runtime ID by name: %w", err)
	}
	var version string
	q := fillEndpointName(qualifier)
	if _, err := strconv.ParseUint(q, 10, 64); err != nil {
		// this case is endpoint name
		version, err = app.GetAgentRuntimeVersionByEndpointName(ctx, *name, q)
		if err != nil {
			return nil, fmt.Errorf("get agent runtime version by endpoint name: %w", err)
		}
	} else {
		version = q
	}
	slog.DebugContext(ctx, "resolved qualifier to version", "qualifier", q, "version", version)
	resp, err := app.ctrlClient.GetAgentRuntime(ctx, &bedrockagentcorecontrol.GetAgentRuntimeInput{
		AgentRuntimeId:      aws.String(id),
		AgentRuntimeVersion: aws.String(version),
	})
	if err != nil {
		var nfe *types.ResourceNotFoundException
		if errors.As(err, &nfe) {
			return nil, ErrAgentRuntimeNotFound
		}
		return nil, fmt.Errorf("GetAgentRuntime: %w", err)
	}
	return resp, nil
}

func (app *App) GetAgentRuntimeIDByName(ctx context.Context, name string) (string, error) {
	app.cacheMu.RLock()
	if id, ok := app.cacheIDbyNames[name]; ok {
		app.cacheMu.RUnlock()
		return id, nil
	}
	app.cacheMu.RUnlock()
	id, _, err := app.findAgentRuntimeByName(ctx, name)
	return id, err
}

func (app *App) GetAgentRuntimeARNByName(ctx context.Context, name string) (string, error) {
	app.cacheMu.RLock()
	if arn, ok := app.cacheARNbyNames[name]; ok {
		app.cacheMu.RUnlock()
		return arn, nil
	}
	app.cacheMu.RUnlock()
	_, arn, err := app.findAgentRuntimeByName(ctx, name)
	return arn, err
}

func (app *App) findAgentRuntimeByName(ctx context.Context, name string) (string, string, error) {
	app.cacheMu.Lock()
	defer app.cacheMu.Unlock()
	slog.DebugContext(ctx, "Fetching AgentRuntimes for resolving name to ID and ARN", "name", name)

	p := bedrockagentcorecontrol.NewListAgentRuntimesPaginator(
		app.ctrlClient,
		&bedrockagentcorecontrol.ListAgentRuntimesInput{},
	)
	for p.HasMorePages() {
		out, err := p.NextPage(ctx)
		if err != nil {
			return "", "", fmt.Errorf("ListAgentRuntimes: %w", err)
		}
		for _, rt := range out.AgentRuntimes {
			slog.DebugContext(ctx, "Found AgentRuntime", "id", *rt.AgentRuntimeId, "name", *rt.AgentRuntimeName)
			if *rt.AgentRuntimeName == name {
				app.cacheIDbyNames[name] = *rt.AgentRuntimeId
				app.cacheARNbyNames[name] = *rt.AgentRuntimeArn
				return *rt.AgentRuntimeId, *rt.AgentRuntimeArn, nil
			}
		}
	}
	slog.DebugContext(ctx, "No AgentRuntime found with the specified name", "name", name)
	return "", "", ErrAgentRuntimeNotFound
}

func (app *App) saveFile(ctx context.Context, path string, b []byte, mode os.FileMode, force bool) error {
	slog.DebugContext(ctx, "writing file", "file", path, "mode", mode)
	if _, err := os.Stat(path); err == nil {
		ok := force || prompter.YN(fmt.Sprintf("Overwrite existing file %s?", path), false)
		if !ok {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return nil
		}
	}
	return os.WriteFile(path, b, mode)
}

func (app *App) loadAgentRuntimeFile(ctx context.Context) (*AgentRuntime, error) {
	path := app.agentRuntimeFilepath
	slog.InfoContext(ctx, "loading agent runtime file", "file", path)
	bs, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}
	ext := filepath.Ext(path)
	if ext == ".jsonnet" {
		jsonStr, err := app.vm.EvaluateAnonymousSnippet(path, string(bs))
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate jsonnet: %w", err)
		}
		bs = []byte(jsonStr)
	}
	def, err := unmarshalAgentRuntime(bs, true)
	if err != nil {
		field := extractUnknownFieldKey(err)
		if field == "" {
			return nil, fmt.Errorf("unmarshalAgentRuntime: %w", err)
		}
		slog.WarnContext(ctx, "unknown field found in agent runtime file", "file", path, "field", extractUnknownFieldKey(err))
		def, err = unmarshalAgentRuntime(bs, false)
		if err != nil {
			return nil, fmt.Errorf("unmarshalAgentRuntime: %w", err)
		}
	}
	return def, validateAgentRuntime(def)
}

func validateAgentRuntime(def *AgentRuntime) error {
	if def == nil {
		return fmt.Errorf("agent runtime definition is nil")
	}
	if def.AgentRuntimeName == nil || *def.AgentRuntimeName == "" {
		return fmt.Errorf("agent runtime name is required")
	}
	return nil
}
