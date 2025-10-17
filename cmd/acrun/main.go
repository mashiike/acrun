package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/mashiike/acrun"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		var exitErr *acrun.ExitError
		if ok := errors.As(err, &exitErr); ok {
			os.Exit(exitErr.Code)
		} else {
			os.Exit(1)
		}
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	var cli acrun.CLI
	return cli.Run(ctx)
}
