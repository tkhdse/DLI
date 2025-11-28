package main

import (
	"context"
	"dli/internal/config"
	"dli/internal/vdb"
	"log"
	"time"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create Pinecone client
	client, err := vdb.NewClient(cfg.PineconeAPIKey, cfg.PineconeIndex)
	if err != nil {
		log.Fatalf("Failed to create Pinecone client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
}
