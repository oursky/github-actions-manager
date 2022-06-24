package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func Run(logger *zap.Logger, modules []Module) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)

	logger.Info("starting...")
	for _, m := range modules {
		if err := m.Start(ctx, g); err != nil {
			return fmt.Errorf("error while starting: %w", err)
		}
	}

	go func() {
		<-sig
		logger.Info("exiting...")
		cancel()
	}()

	return g.Wait()
}
