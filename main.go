package main

import (
	"context"
	"log"
	"os"
	"os/signal"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Printf("configuration error: %v", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := newApp(cfg).run(ctx); err != nil {
		log.Printf("application error: %v", err)
		os.Exit(1)
	}
}
