package main

import (
	"context"
	"dli/internal/embedding"
	"fmt"
	"log"
	"time"
)

func main() {
	var server_address string = "localhost:50051"
	fmt.Printf("Connecting to embedding server at %s...\n", server_address)

	// Create client
	client, err := embedding.NewClient(server_address)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	defer client.Close()

	fmt.Println("Connected to embedding server")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// test GetEmbedding()
	prompt := "Random Prompt"
	fmt.Printf("Generating Embedding for prompt: %s\n", prompt)
	embedding, err := client.GetEmbedding(ctx, prompt)
	if err != nil {
		log.Fatalf("Failed to get embedding: %v", err)
	}

	fmt.Printf("Resulting Embedding: %v\n", embedding[:min(5, len(embedding))])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
