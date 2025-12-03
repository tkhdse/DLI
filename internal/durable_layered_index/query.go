package main

import (
	"context"
	"dli/embedding"
	"fmt"
	"log"
	"sync"
)

var (
	embeddingClient *embedding.Client
	clientOnce      sync.Once
	clientErr       error
)

// InitEmbeddingClient initializes the global embedding client
// Call this once at application startup
func InitEmbeddingClient(serverAddress string) error {
	clientOnce.Do(func() {
		log.Printf("Initializing embedding client for %s...", serverAddress)
		embeddingClient, clientErr = embedding.NewClient(serverAddress)
		if clientErr != nil {
			log.Printf("Failed to create embedding client: %v", clientErr)
			return
		}
		log.Printf("Embedding client initialized successfully")
	})
	return clientErr
}

// CloseEmbeddingClient closes the global embedding client
func CloseEmbeddingClient() {
	if embeddingClient != nil {
		log.Println("Closing embedding client...")
		embeddingClient.Close()
	}
}

// Query represents a single query with its text, embedding, and result channel
type Query struct {
	Text      string
	Embedding []float32
	ResultCh  chan string
}

// NewQuery creates a new query with the given text
// Uses the gRPC embedding service to generate the embedding
func NewQuery(ctx context.Context, text string) (*Query, error) {
	if embeddingClient == nil {
		return nil, fmt.Errorf("embedding client not initialized - call InitEmbeddingClient first")
	}

	// Get real embedding from gRPC server
	embedding, err := embeddingClient.GetEmbedding(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("failed to get embedding for '%s': %w", text, err)
	}

	return &Query{
		Text:      text,
		Embedding: embedding,
		ResultCh:  make(chan string, 1),
	}, nil
}

// NewQueryWithEmbedding creates a query with a pre-computed embedding
// Useful if you already have the embedding and want to avoid the gRPC call
func NewQueryWithEmbedding(text string, embedding []float32) *Query {
	return &Query{
		Text:      text,
		Embedding: embedding,
		ResultCh:  make(chan string, 1),
	}
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
