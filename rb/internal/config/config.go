package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	// Embedding service config
	EmbeddingServerAddr string

	// Pinecone config
	PineconeAPIKey string
	PineconeIndex  string
	PineconeRegion string
}

func Load() (*Config, error) {
	_ = godotenv.Load()
	cfg := &Config{
		EmbeddingServerAddr: getEnv("EMBEDDING_SERVER_ADDR", "localhost:50051"),
		PineconeAPIKey:      getEnv("PINECONE_KEY", ""),
		PineconeIndex:       getEnv("PINECONE_INDEX", ""),
		PineconeRegion:      getEnv("PINECONE_REGION", ""),
	}

	// Validate required fields
	if cfg.PineconeAPIKey == "" {
		return nil, fmt.Errorf("PINECONE_API_KEY is required")
	}
	if cfg.PineconeIndex == "" {
		return nil, fmt.Errorf("PINECONE_INDEX is required")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
