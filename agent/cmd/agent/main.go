package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/robot-remote-maint/agent/internal/client"
	"github.com/robot-remote-maint/agent/internal/config"
	"github.com/robot-remote-maint/agent/pkg/logger"
)

func main() {
	cfg, err := config.Load("agent.env")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	lg := logger.New(cfg.LogLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		lg.Info("Received shutdown signal")
		cancel()
	}()

	wsClient := client.New(cfg, lg)
	if err := wsClient.Connect(ctx); err != nil {
		lg.Error("Failed to connect", "error", err)
		log.Fatalf("Failed to connect: %v", err)
	}

	<-ctx.Done()
	lg.Info("Agent shutting down...")
	wsClient.Close()
}
