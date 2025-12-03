package main

import (
	"context"
	"math/rand"
)

// Query represents a single query with its text, embedding, and result channel
type Query struct {
	Text      string
	Embedding []float32
	ResultCh  chan string
}

// NewQuery creates a new query with the given text
// In production, integrate with an actual embedding model here
func NewQuery(ctx context.Context, text string) *Query {
	embedding := generateDummyEmbedding(384)

	return &Query{
		Text:      text,
		Embedding: embedding,
		ResultCh:  make(chan string, 1),
	}
}

// generateDummyEmbedding creates a placeholder embedding
// Replace this with actual sentence transformer integration
func generateDummyEmbedding(dim int) []float32 {
	emb := make([]float32, dim)
	for i := range emb {
		emb[i] = rand.Float32()
	}
	normalizeVector(emb)
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
