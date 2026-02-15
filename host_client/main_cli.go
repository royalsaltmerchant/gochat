package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	cfg, err := LoadOrInitHostConfigCLI()
	if err != nil {
		log.Fatalf("Failed to load host config: %v", err)
	}

	fmt.Println("========================================")
	fmt.Println("Parch Host CLI")
	fmt.Println("========================================")
	fmt.Printf("Host Name: %s\n", cfg.Name)
	fmt.Printf("Host UUID: %s\n", cfg.UUID)
	fmt.Println("========================================")
	fmt.Println("Press Ctrl+C to shutdown")
	fmt.Println()

	ctx, cancel := context.WithCancel(context.Background())

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutdown signal received...")
		cancel()
	}()

	runMainLogic(ctx, cfg)
}

func promptInput(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}
