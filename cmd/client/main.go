package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"qtrp/internal/client"
	"qtrp/pkg/config"
)

func main() {
	configPath := flag.String("c", "client.yaml", "config file path")
	flag.Parse()

	log.Printf("[main] loading config from: %s", *configPath)
	cfg, err := config.LoadClientConfig(*configPath)
	if err != nil {
		log.Fatalf("[main] load config error: %v", err)
	}

	log.Printf("[main] creating client instance")
	cli, err := client.New(cfg)
	if err != nil {
		log.Fatalf("[main] create client error: %v", err)
	}

	// 优雅退出
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("[main] received signal: %v, shutting down...", sig)
		cli.Stop()
		log.Printf("[main] client stopped gracefully")
		os.Exit(0)
	}()

	log.Printf("[main] starting client")
	if err := cli.Run(); err != nil {
		log.Fatalf("[main] client error: %v", err)
	}
}
