package acrun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aereal/jsondiff"
	"github.com/fatih/color"
	"github.com/itchyny/gojq"
)

type DiffOption struct {
	Qualifier *string `help:"the qualifier to compare; allow endpoint name or version number"`
	Ignore    string  `help:"ignore diff by jq query" default:""`
	ExitCode  bool    `help:"exit with code 2 if there are differences" default:"false"`
}

func coloredDiff(src string) string {
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(line, "-") {
			b.WriteString(color.RedString(line) + "\n")
		} else if strings.HasPrefix(line, "+") {
			b.WriteString(color.GreenString(line) + "\n")
		} else {
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

func (app *App) Diff(ctx context.Context, opt *DiffOption) error {
	local, err := app.loadAgentRuntimeFile(ctx)
	if err != nil {
		return fmt.Errorf("load agent runtime file: %w", err)
	}
	var remote *AgentRuntime
	resp, err := app.GetAgentRuntime(ctx, local.AgentRuntimeName, opt.Qualifier)
	if err != nil {
		if !errors.Is(err, ErrAgentRuntimeNotFound) {
			return fmt.Errorf("get remote agent runtime: %w", err)
		}
		slog.InfoContext(
			ctx,
			fmt.Sprintf("remote AgentRuntime not found, %s deploy will create a new agent runtime",
				AppName,
			),
			"agent_runtime_name", *local.AgentRuntimeName,
		)
	} else {
		remote, err = newAgentRuntimeFromResponse(resp)
		if err != nil {
			return fmt.Errorf("newAgentRuntimeFromResponse: %w", err)
		}
	}

	opts := []jsondiff.Option{}
	if ignore := opt.Ignore; ignore != "" {
		if p, err := gojq.Parse(ignore); err != nil {
			return fmt.Errorf("failed to parse ignore query: %s %w", ignore, err)
		} else {
			opts = append(opts, jsondiff.Ignore(p))
		}
	}

	var remoteAny, localAny interface{}
	remoteJSON, err := marshalAgentRuntime(remote, "  ")
	if err != nil {
		return fmt.Errorf("marshalAgentRuntime: %w", err)
	}
	if err := json.Unmarshal(remoteJSON, &remoteAny); err != nil {
		return fmt.Errorf("unmarshal remote agent runtime: %w", err)
	}
	localJSON, err := marshalAgentRuntime(local, "  ")
	if err != nil {
		return fmt.Errorf("marshalAgentRuntime: %w", err)
	}
	if err := json.Unmarshal(localJSON, &localAny); err != nil {
		return fmt.Errorf("unmarshal local agent runtime: %w", err)
	}
	remoteARN := "(known after deploy)"
	remoteVersion := ""
	if remote != nil && resp != nil && resp.AgentRuntimeArn != nil {
		remoteARN = *resp.AgentRuntimeArn
		switch {
		case resp.AgentRuntimeVersion != nil:
			remoteVersion = "Version " + *resp.AgentRuntimeVersion
		case opt.Qualifier != nil:
			remoteVersion = *opt.Qualifier
		}
	}
	hasDiff := false
	if diff, err := jsondiff.Diff(
		&jsondiff.Input{Name: remoteARN + ";" + remoteVersion, X: remoteAny},
		&jsondiff.Input{Name: app.agentRuntimeFilepath, X: localAny},
		opts...,
	); err != nil {
		return fmt.Errorf("failed to diff: %w", err)
	} else if diff != "" {
		hasDiff = true
		fmt.Print(coloredDiff(diff))
	} else {
		slog.InfoContext(ctx, "no differences found", "agent_runtime_name", *local.AgentRuntimeName, "remote_arn", remoteARN, "local_file", app.agentRuntimeFilepath)
	}
	if hasDiff && opt.ExitCode {
		return ErrDiff
	}
	return nil
}
