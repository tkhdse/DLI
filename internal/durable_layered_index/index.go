package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// DurableLayeredIndex manages bins and routes queries based on vector similarity
type DurableLayeredIndex struct {
	dbCollection      interface{}
	binVectors        [][]float32 // Representative vector for each bin
	bins              []*Bin
	mu                sync.RWMutex
	groupingThreshold float32 // Similarity threshold for grouping (e.g., 0.85)
	maxBinsPerIndex   int     // Maximum number of bins to prevent unbounded growth
}

// NewDurableLayeredIndex creates a new index with the specified configuration
func NewDurableLayeredIndex(dbCollection interface{}, maxBins int, groupingThreshold float32) *DurableLayeredIndex {
	if maxBins <= 0 {
		maxBins = 100 // Default max bins
	}

	dli := &DurableLayeredIndex{
		dbCollection:      dbCollection,
		binVectors:        make([][]float32, 0),
		bins:              make([]*Bin, 0),
		groupingThreshold: groupingThreshold,
		maxBinsPerIndex:   maxBins,
	}

	log.Printf("Initialized DLI with grouping threshold: %.2f, max bins: %d", groupingThreshold, maxBins)

	return dli
}

// addBinWithVector creates a new bin with a specific representative vector
func (dli *DurableLayeredIndex) addBinWithVector(vector []float32) int {
	dli.mu.Lock()
	defer dli.mu.Unlock()

	// Create a copy of the vector
	binVector := make([]float32, len(vector))
	copy(binVector, vector)

	dli.binVectors = append(dli.binVectors, binVector)
	dli.bins = append(dli.bins, NewBin(dli.dbCollection, 6, 500*time.Millisecond))

	binIndex := len(dli.bins) - 1
	log.Printf("Created bin %d (total bins: %d)", binIndex, len(dli.bins))

	return binIndex
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

// addQueryToBin finds the best matching bin or creates a new one if needed
func (dli *DurableLayeredIndex) addQueryToBin(query *Query) error {
	dli.mu.RLock()
	numBins := len(dli.bins)
	dli.mu.RUnlock()

	// If no bins exist, create the first one
	if numBins == 0 {
		log.Printf("No bins exist, creating first bin for query: '%s'", query.Text)
		binIdx := dli.addBinWithVector(query.Embedding)
		dli.mu.RLock()
		dli.bins[binIdx].AddQuery(query)
		dli.mu.RUnlock()
		return nil
	}

	// Find the most similar bin
	dli.mu.RLock()
	bestBinIdx := 0
	bestSimilarity := float32(-1.0)

	for i, binVector := range dli.binVectors {
		similarity := cosineSimilarity(query.Embedding, binVector)
		if similarity > bestSimilarity {
			bestSimilarity = similarity
			bestBinIdx = i
		}
	}

	selectedBin := dli.bins[bestBinIdx]
	canCreateNewBin := len(dli.bins) < dli.maxBinsPerIndex
	dli.mu.RUnlock()

	// Decide whether to use existing bin or create new one
	if bestSimilarity >= dli.groupingThreshold {
		// Query is similar enough to existing bin
		log.Printf("Query '%s' assigned to bin %d (similarity: %.3f)",
			query.Text, bestBinIdx, bestSimilarity)
		selectedBin.AddQuery(query)
	} else if canCreateNewBin {
		// Query is too different, create new bin
		log.Printf("Query '%s' too different (similarity: %.3f < threshold: %.3f), creating new bin",
			query.Text, bestSimilarity, dli.groupingThreshold)
		binIdx := dli.addBinWithVector(query.Embedding)
		dli.mu.RLock()
		dli.bins[binIdx].AddQuery(query)
		dli.mu.RUnlock()
	} else {
		// Max bins reached, use closest bin anyway
		log.Printf("Max bins reached (%d), assigning query '%s' to closest bin %d (similarity: %.3f)",
			dli.maxBinsPerIndex, query.Text, bestBinIdx, bestSimilarity)
		selectedBin.AddQuery(query)
	}

	return nil
}

// GetStats returns statistics about the index
func (dli *DurableLayeredIndex) GetStats() map[string]interface{} {
	dli.mu.RLock()
	defer dli.mu.RUnlock()

	return map[string]interface{}{
		"num_bins":           len(dli.bins),
		"grouping_threshold": dli.groupingThreshold,
		"max_bins":           dli.maxBinsPerIndex,
	}
}

// Close shuts down all bins gracefully
func (dli *DurableLayeredIndex) Close(ctx context.Context) error {
	dli.mu.RLock()
	bins := dli.bins
	dli.mu.RUnlock()

	log.Printf("Shutting down %d bins...", len(bins))

	for i, bin := range bins {
		if err := bin.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown bin %d: %w", i, err)
		}
	}

	log.Println("All bins shut down successfully")
	return nil
}
