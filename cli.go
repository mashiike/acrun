package acrun

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/fatih/color"
	"github.com/mashiike/slogutils"
)

type CLI struct {
	GlobalOption
	Color     bool   `help:"enable colored output" default:"true" env:"ACRUN_COLOR" negatable:"" json:"color,omitempty"`
	LogLevel  string `help:"Log level" default:"info" enum:"debug,info,warn,error"`
	LogFormat string `help:"Log format(text,json)" default:"text" enum:"text,json"`

	Init     InitOption     `cmd:"" help:"Initialize acrun configuration."`
	Invoke   InvokeOption   `cmd:"" help:"Invoke the agent."`
	Diff     DiffOption     `cmd:"" help:"Diff the local and remote agent runtime."`
	Deploy   DeployOption   `cmd:"" help:"Deploy the agent runtime."`
	Render   RenderOption   `cmd:"" help:"Render the agent runtime configuration."`
	Delete   DeleteOption   `cmd:"" help:"Delete the agent runtime."`
	Rollback RollbackOption `cmd:"" help:"Rollback the agent runtime to a specific version."`
	Version  struct{}       `cmd:"" help:"Show version."`
}

func (c *CLI) Run(ctx context.Context) error {
	k := kong.Parse(c, kong.Vars{"version": fmt.Sprintf("acrun %s", Version)}, kong.Name(AppName))
	if strings.Split(k.Command(), " ")[0] == "version" {
		fmt.Fprintf(os.Stdout, "acrun %s\n", Version)
		return nil
	}
	var logLevel slog.Level
	if err := logLevel.UnmarshalText([]byte(c.LogLevel)); err != nil {
		return err
	}
	opts := slogutils.MiddlewareOptions{
		ModifierFuncs: map[slog.Level]slogutils.ModifierFunc{
			slog.LevelDebug: slogutils.Color(color.FgBlack),
			slog.LevelInfo:  nil,
			slog.LevelWarn:  slogutils.Color(color.FgYellow),
			slog.LevelError: slogutils.Color(color.FgRed, color.Bold),
		},
		Writer: os.Stderr,
		HandlerOptions: &slog.HandlerOptions{
			Level: logLevel,
		},
	}
	var logger *slog.Logger
	switch c.LogFormat {
	case "text":
		logger = slog.New(slogutils.NewMiddleware(
			slog.NewTextHandler,
			opts,
		))
	case "json":
		logger = slog.New(slogutils.NewMiddleware(
			slog.NewJSONHandler,
			opts,
		))
	default:
		return fmt.Errorf("unknown log format: %s", c.LogFormat)
	}
	slog.SetDefault(logger)
	color.NoColor = !c.Color

	app, err := New(ctx, &c.GlobalOption)
	if err != nil {
		return err
	}
	switch strings.Split(k.Command(), " ")[0] {
	case "init":
		return app.Init(ctx, &c.Init)
	case "invoke":
		return app.Invoke(ctx, &c.Invoke)
	case "diff":
		return app.Diff(ctx, &c.Diff)
	case "deploy":
		return app.Deploy(ctx, &c.Deploy)
	case "render":
		return app.Render(ctx, &c.Render)
	case "delete":
		return app.Delete(ctx, &c.Delete)
	case "rollback":
		return app.Rollback(ctx, &c.Rollback)
	default:
		return fmt.Errorf("unknown command: %s", k.Command())
	}
}
