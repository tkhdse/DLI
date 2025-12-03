package main

import (
	"context"
)

// Query represents a single query with its text, embedding, and result channel
type Query struct {
	Text      string
	Embedding []float32
	ResultCh  chan string // Channel to receive the result
}

// NewQuery creates a new query with the given text
// In production, you'd integrate with an actual embedding model here
func NewQuery(ctx context.Context, text string) *Query {
	// TODO: Replace with actual embedding generation
	// For now, creating a dummy 384-dimensional embedding
	embedding := generateDummyEmbedding(384)

	return &Query{
		Text:      text,
		Embedding: embedding,
		ResultCh:  make(chan string, 1), // Buffered channel for result
	}
}

// generateDummyEmbedding creates a placeholder embedding
// Replace this with actual sentence transformer integration
func generateDummyEmbedding(dim int) []float32 {
	emb := make([]float32, dim)
	for i := range emb {
		emb[i] = 0.1 // Placeholder values
	}
	return emb
}

// GetResult waits for and returns the query result
func (q *Query) GetResult(ctx context.Context) (string, error) {
	select {
	case result := <-q.ResultCh:
		return result, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// SetResult sends a result to the query's channel
func (q *Query) SetResult(result string) {
	select {
	case q.ResultCh <- result:
	default:
		// Channel already has a result
	}
}
