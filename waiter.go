package acrun

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

type Waiter struct {
	MaxDuration   time.Duration
	CheckInterval time.Duration
	LogInterval   time.Duration
	LogMessage    string
	LogAttributes []any
	Checker       func(context.Context) ([]any, bool, error)
}

func (w *Waiter) Wait(ctx context.Context) error {
	if w.LogMessage == "" {
		return errors.New("waiter: LogMessage is required")
	}
	if w.MaxDuration == 0 {
		w.MaxDuration = 30 * time.Minute
	}
	if w.CheckInterval == 0 {
		w.CheckInterval = 5 * time.Second
	}
	if w.LogInterval == 0 {
		w.LogInterval = 1 * time.Minute
	}
	if w.LogInterval < w.CheckInterval {
		w.LogInterval = w.CheckInterval
	}
	deadlineCtx, cancel := context.WithTimeout(ctx, w.MaxDuration)
	defer cancel()
	ticker := time.NewTicker(w.CheckInterval)
	defer ticker.Stop()
	logTicker := time.NewTicker(w.LogInterval)
	defer logTicker.Stop()
	currentAttrs := append([]any{}, w.LogAttributes...)
	for {
		select {
		case <-deadlineCtx.Done():
			return errors.New("waiter: context deadline exceeded")
		case <-ticker.C:
			attrs, done, err := w.Checker(ctx)
			if err != nil {
				return err
			}
			if len(attrs) > 0 {
				currentAttrs = append([]any{}, w.LogAttributes...)
				currentAttrs = append(currentAttrs, attrs...)
			}
			if done {
				return nil
			}
		case <-logTicker.C:
			slog.InfoContext(ctx, w.LogMessage, currentAttrs...)
		}
	}
}
