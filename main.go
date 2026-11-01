package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tfkfan/gogame/internal/config"
	"github.com/tfkfan/gogame/internal/game/server"
)

func main() {
	log.SetFlags(0)

	err := run(context.Background())
	if err != nil {
		log.Fatal(err)
	}
}

func run(parent context.Context) error {
	cfg, e := config.Read()
	if e != nil {
		return e
	}

	srv, errc, e := server.NewGameServer(cfg)
	if e != nil {
		return e
	}

	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-parent.Done():
	case err := <-errc:
		log.Printf("failed to serve: %v", err)
	case sig := <-sigs:
		log.Printf("terminating: %v", sig)
	}

	ctx, cancel := context.WithTimeout(parent, time.Second*10)
	defer cancel()

	return srv.Shutdown(ctx)
}
