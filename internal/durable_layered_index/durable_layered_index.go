package main

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// DurableLayeredIndex manages bins and routes queries
type DurableLayeredIndex struct {
	dbCollection      interface{}
	binVectors        [][]float32
	bins              []*Bin
	mu                sync.RWMutex
	groupingThreshold float32
}

// NewDurableLayeredIndex creates a new index
func NewDurableLayeredIndex(dbCollection interface{}, numBins int, groupingThreshold float32) *DurableLayeredIndex {
	dli := &DurableLayeredIndex{
		dbCollection:      dbCollection,
		binVectors:        make([][]float32, 0),
		bins:              make([]*Bin, 0),
		groupingThreshold: groupingThreshold,
	}

	// Add initial bin
	dli.addBin()

	return dli
}

// addBin creates a new bin with a random vector
func (dli *DurableLayeredIndex) addBin() {
	// Generate random bin vector (384 dimensions)
	binVector := make([]float32, 384)
	for i := range binVector {
		binVector[i] = rand.Float32()
	}

	// Normalize the vector
	normalizeVector(binVector)

	dli.mu.Lock()
	defer dli.mu.Unlock()

	dli.binVectors = append(dli.binVectors, binVector)
	dli.bins = append(dli.bins, NewBin(dli.dbCollection, 6, 500*time.Millisecond))
}

// Query processes a query asynchronously
func (dli *DurableLayeredIndex) Query(ctx context.Context, queryText string) (string, error) {
	// Create query with embedding
	query := NewQuery(ctx, queryText)

	// Add query to appropriate bin
	if err := dli.addQueryToBin(query); err != nil {
		return "", err
	}

	// Wait for result
	return query.GetResult(ctx)
}

// addQueryToBin finds the best matching bin and adds the query
func (dli *DurableLayeredIndex) addQueryToBin(query *Query) error {
	dli.mu.RLock()
	defer dli.mu.RUnlock()

	// Find closest bin using cosine similarity
	bestBinIdx := 0
	bestSimilarity := float32(-1.0)

	for i, binVector := range dli.binVectors {
		similarity := cosineSimilarity(query.Embedding, binVector)
		if similarity > bestSimilarity {
			bestSimilarity = similarity
			bestBinIdx = i
		}
	}

	// Add query to the selected bin
	dli.bins[bestBinIdx].AddQuery(query)

	return nil
}

// Close shuts down all bins gracefully
func (dli *DurableLayeredIndex) Close(ctx context.Context) error {
	dli.mu.RLock()
	bins := dli.bins
	dli.mu.RUnlock()

	for _, bin := range bins {
		if err := bin.Shutdown(ctx); err != nil {
			return err
		}
	}

	return nil
}

// Helper functions

// cosineSimilarity calculates cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float32

	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

// normalizeVector normalizes a vector in place
func normalizeVector(v []float32) {
	var norm float32
	for _, val := range v {
		norm += val * val
	}
	norm = float32(math.Sqrt(float64(norm)))

	if norm > 0 {
		for i := range v {
			v[i] /= norm
		}
	}
}

// Example usage
func main() {
	ctx := context.Background()

	// Create index
	dli := NewDurableLayeredIndex(nil, 1, 0.9)
	defer dli.Close(ctx)

	// Create queries
	queries := []string{"Q1", "Q2", "Q3", "Q4", "Q5", "Q6"}

	// Process queries concurrently
	results := make([]string, len(queries))
	var wg sync.WaitGroup

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

	wg.Wait()

	// Print results
	fmt.Println("Results:")
	for i, result := range results {
		fmt.Printf("%d: %s\n", i, result)
	}
}
