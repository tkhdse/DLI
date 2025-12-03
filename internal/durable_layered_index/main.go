package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	// Configuration
	embeddingServerAddr := "localhost:50051"

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal, shutting down gracefully...")
		cancel()
	}()

	// Initialize embedding client
	fmt.Printf("Connecting to embedding server at %s...\n", embeddingServerAddr)
	if err := InitEmbeddingClient(embeddingServerAddr); err != nil {
		log.Fatalf("Failed to initialize embedding client: %v", err)
	}
	defer CloseEmbeddingClient()
	fmt.Println("✓ Connected to embedding server")

	// Initialize the Durable Layered Index
	fmt.Println("\nInitializing Durable Layered Index...")
	dli := NewDurableLayeredIndex(nil, 1, 0.9)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		dli.Close(shutdownCtx)
	}()
	fmt.Println("✓ DLI initialized")

	// Define test queries
	queries := []string{
		"What is machine learning?",
		"Explain neural networks",
		"How does deep learning work?",
		"What is artificial intelligence?",
		"Define supervised learning",
		"What are transformers in AI?",
	}

	line := strings.Repeat("=", 60)
	fmt.Printf("\n" + line + "\n")
	fmt.Printf("Processing %d queries concurrently...\n", len(queries))
	fmt.Printf(line + "\n\n")

	// Process queries concurrently
	results := make([]string, len(queries))
	errors := make([]error, len(queries))
	var wg sync.WaitGroup

	startTime := time.Now()

	for i, q := range queries {
		wg.Add(1)
		go func(idx int, queryText string) {
			defer wg.Done()

			// Create context with timeout for individual query
			queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			queryStart := time.Now()
			result, err := dli.Query(queryCtx, queryText)
			queryDuration := time.Since(queryStart)

			if err != nil {
				errors[idx] = err
				fmt.Printf("✗ Query %d failed after %v: %v\n", idx+1, queryDuration, err)
				return
			}
			results[idx] = result
			fmt.Printf("✓ Query %d completed in %v\n", idx+1, queryDuration)
		}(i, q)
	}

	// Wait for all queries to complete
	wg.Wait()

	totalDuration := time.Since(startTime)

	// Count successes and failures
	successCount := 0
	for _, err := range errors {
		if err == nil {
			successCount++
		}
	}

	// Print detailed results

	fmt.Println("\n" + line)
	fmt.Println("RESULTS")
	fmt.Println(line)
	for i, result := range results {
		if errors[i] != nil {
			fmt.Printf("\n[%d] ✗ FAILED\n", i+1)
			fmt.Printf("Query: %s\n", queries[i])
			fmt.Printf("Error: %v\n", errors[i])
		} else {
			fmt.Printf("\n[%d] ✓ SUCCESS\n", i+1)
			fmt.Printf("Query: %s\n", queries[i])
			fmt.Printf("Result: %s\n", result)
		}
	}

	// Print summary statistics
	fmt.Println("\n" + line)
	fmt.Println("SUMMARY")
	fmt.Println(line)
	fmt.Printf("Total queries: %d\n", len(queries))
	fmt.Printf("Successful: %d\n", successCount)
	fmt.Printf("Failed: %d\n", len(queries)-successCount)
	fmt.Printf("Total time: %v\n", totalDuration)
	fmt.Printf("Average time per query: %v\n", totalDuration/time.Duration(len(queries)))
	fmt.Println(line)
}
