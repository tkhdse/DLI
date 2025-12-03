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
	// Parameters: dbCollection, maxBins, groupingThreshold
	// Lower threshold = more bins (stricter grouping)
	// Higher threshold = fewer bins (looser grouping)
	fmt.Println("\nInitializing Durable Layered Index...")
	dli := NewDurableLayeredIndex(nil, 100, 0.7) // 0.85 threshold for moderate grouping
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		dli.Close(shutdownCtx)
	}()
	fmt.Println("✓ DLI initialized")

	// Define test queries - grouped by topic to see if DLI groups them correctly
	queries := []string{
		// Group 1: Machine Learning basics
		"What is machine learning?",
		"Explain machine learning algorithms",
		"How does machine learning work?",

		// Group 2: Deep Learning
		"What is deep learning?",
		"Explain neural networks",
		"How do neural networks learn?",

		// Group 3: Natural Language Processing
		"What is natural language processing?",
		"How does NLP work?",
		"Explain text processing",

		// Group 4: Computer Vision
		"What is computer vision?",
		"How does image recognition work?",
		"Explain object detection",

		// Group 5: Unrelated - Cooking
		"How do I bake a cake?",
		"What is the best chocolate recipe?",

		// Group 6: Unrelated - Sports
		"Who won the world cup?",
		"Explain basketball rules",
	}

	line := strings.Repeat("=", 60)
	fmt.Printf("\n" + line + "\n")
	fmt.Printf("Processing %d queries to test similarity grouping...\n", len(queries))
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

	// Print DLI statistics
	stats := dli.GetStats()
	fmt.Println("\n" + line)
	fmt.Println("DLI STATISTICS")
	fmt.Println(line)
	fmt.Printf("Total bins created: %v\n", stats["num_bins"])
	fmt.Printf("Grouping threshold: %.2f\n", stats["grouping_threshold"])
	fmt.Printf("Max bins allowed: %v\n", stats["max_bins"])

	// Print summary
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
