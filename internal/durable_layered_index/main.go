package main

import (
	"context"
	"fmt"
	"sync"
)

func main() {
	// Create a background context
	ctx := context.Background()

	// Initialize the Durable Layered Index
	// Parameters: dbCollection, numBins, groupingThreshold
	dli := NewDurableLayeredIndex(nil, 1, 0.9)
	defer dli.Close(ctx)

	// Define test queries
	queries := []string{"Q1", "Q2", "Q3", "Q4", "Q5", "Q6"}

	// Process queries concurrently
	results := make([]string, len(queries))
	var wg sync.WaitGroup

	fmt.Println("Processing queries...")

	for i, q := range queries {
		wg.Add(1)
		go func(idx int, queryText string) {
			defer wg.Done()

			result, err := dli.Query(ctx, queryText)
			if err != nil {
				fmt.Printf("Error processing query %s: %v\n", queryText, err)
				return
			}
			results[idx] = result
		}(i, q)
	}

	// Wait for all queries to complete
	wg.Wait()

	// Print results
	fmt.Println("\nResults:")
	for i, result := range results {
		fmt.Printf("%d: %s\n", i, result)
	}
}
