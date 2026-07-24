package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"codeatlas/apps/webhook-service/internal/app"
	serviceconfig "codeatlas/apps/webhook-service/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := serviceconfig.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	application, err := app.New(ctx, cfg)
	if err != nil {
		log.Fatalf("create app: %v", err)
	}

	if err := application.Run(ctx); err != nil {
		log.Fatalf("run app: %v", err)
	}
}
