package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"qtrp/internal/server"
	"qtrp/pkg/config"
)

func main() {
	configPath := flag.String("c", "server.yaml", "config file path")
	flag.Parse()

	log.Printf("[main] loading config from: %s", *configPath)
	cfg, err := config.LoadServerConfig(*configPath)
	if err != nil {
		log.Fatalf("[main] load config error: %v", err)
	}

	log.Printf("[main] creating server instance")
	srv := server.New(cfg)

	// 优雅退出
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("[main] received signal: %v, shutting down...", sig)
		if err := srv.Stop(); err != nil {
			log.Printf("[main] stop server error: %v", err)
		}
		os.Exit(0)
	}()

	log.Printf("[main] starting server")
	if err := srv.Start(); err != nil {
		log.Fatalf("[main] server error: %v", err)
	}
}
