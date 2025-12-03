package main

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// DurableLayeredIndex manages bins and routes queries based on vector similarity
type DurableLayeredIndex struct {
	dbCollection      interface{}
	binVectors        [][]float32
	bins              []*Bin
	mu                sync.RWMutex
	groupingThreshold float32
}

// NewDurableLayeredIndex creates a new index with the specified configuration
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

// addBin creates a new bin with a representative vector
// In production, you might want to use a real embedding as the bin vector
func (dli *DurableLayeredIndex) addBin() {
	// Generate random bin vector (384 dimensions to match embedding size)
	// TODO: Consider using a representative embedding from actual queries
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

// Query processes a query asynchronously and returns the result
func (dli *DurableLayeredIndex) Query(ctx context.Context, queryText string) (string, error) {
	// Create query with embedding from gRPC service
	query, err := NewQuery(ctx, queryText)
	if err != nil {
		return "", fmt.Errorf("failed to create query: %w", err)
	}

	// Add query to appropriate bin
	if err := dli.addQueryToBin(query); err != nil {
		return "", fmt.Errorf("failed to add query to bin: %w", err)
	}

	// Wait for and return result
	return query.GetResult(ctx)
}

// addQueryToBin finds the best matching bin using cosine similarity and adds the query
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
