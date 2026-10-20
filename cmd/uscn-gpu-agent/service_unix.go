//go:build darwin || linux

package main

import (
	"context"
	"github.com/beijingcao/uscn-gpu-agent/internal/gpuagent"
	"os"
	"os/signal"
	"syscall"
)

func runService(path string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return gpuagent.Run(ctx, path, gpuagent.Options{})
}
