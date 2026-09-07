package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/GLEZH/gofermart-graduate-project/internal/app"
	"github.com/GLEZH/gofermart-graduate-project/internal/config"
)

func main() {
	cfg, err := config.New(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	application, err := app.New(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer application.Close()
	if err = application.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
