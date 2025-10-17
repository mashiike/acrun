package acrun

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

type InitOption struct {
	AgentRuntimeName string  `help:"AgentRuntime name." required:"true"`
	Qualifier        *string `help:"the qualifier to initialize. if not specified, use the latest version." default:""`
	Format           string  `help:"Output format. json or jsonnet" default:"jsonnet" enum:"json,jsonnet"`
	ForceOverwrite   bool    `help:"Overwrite existing files without prompting" default:"false"`
}

func (app *App) Init(ctx context.Context, opt *InitOption) error {
	resp, err := app.GetAgentRuntime(ctx, &opt.AgentRuntimeName, opt.Qualifier)
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "fetched AgentRuntime", "name", opt.AgentRuntimeName, "arn", resp.AgentRuntimeArn)
	def, err := newAgentRuntimeFromResponse(resp)
	if err != nil {
		return fmt.Errorf("newAgentRuntimeFromResponse: %w", err)
	}
	bs, err := marshalAgentRuntime(def, "  ")
	if err != nil {
		return err
	}
	var filename string
	if opt.Format == "jsonnet" {
		bs, err = jsonToJsonnet(bs, "agent_runtime.jsonnet")
		if err != nil {
			return fmt.Errorf("jsonToJsonnet: %w", err)
		}
		filename = DefaultAgentRuntimeFilenames[1]
	} else {
		filename = DefaultAgentRuntimeFilenames[0]
	}
	slog.InfoContext(ctx, "createing agent runtime file", "file", filename)
	if err := app.saveFile(ctx, filename, bs, os.FileMode(0644), opt.ForceOverwrite); err != nil {
		return fmt.Errorf("saveFile: %w", err)
	}
	return nil
}
