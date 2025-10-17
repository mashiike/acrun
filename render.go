package acrun

import (
	"context"
	"fmt"
)

type RenderOption struct {
	Format string `help:"output format (json, jsonnet)" default:"json" enum:"json,jsonnet"`
}

func (app *App) Render(ctx context.Context, opt *RenderOption) error {
	agentRuntime, err := app.loadAgentRuntimeFile(ctx)
	if err != nil {
		return fmt.Errorf("load agent runtime file: %w", err)
	}

	output, err := marshalAgentRuntime(agentRuntime, "  ")
	if err != nil {
		return fmt.Errorf("marshal agent runtime: %w", err)
	}
	switch opt.Format {
	case "json":
		// do nothing
	case "jsonnet":
		output, err = jsonToJsonnet(output, app.agentRuntimeFilepath)
		if err != nil {
			return fmt.Errorf("convert to jsonnet: %w", err)
		}
	default:
		return fmt.Errorf("unsupported format: %s", opt.Format)
	}

	fmt.Fprintln(app.stdout, string(output))
	return nil
}
