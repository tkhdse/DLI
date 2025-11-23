package embedding

import (
	context "context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

type Client struct {
	conn   *grpc.ClientConn
	client EmbeddingServiceClient
}

// Create a new client to connect with Embedding server
func NewClient(serverAddr string) (*Client, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             3 * time.Second,
			PermitWithoutStream: true,
		}),
	}

	conn, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to embeddings service: %w", err)
	}

	client := NewEmbeddingServiceClient(conn)
	return &Client{
		conn:   conn,
		client: client,
	}, nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) GetEmbedding(ctx context.Context, prompt string) ([]float32, error) {
	req := EmbeddingRequest{
		Prompt: prompt,
	}

	resp, err := c.client.GetEmbedding(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("failed to get embedding %w", err)
	}

	return resp.Embedding, nil
}

func (c *Client) GetEmbeddingBatch(ctx context.Context, prompts []string) ([][]float32, error) {
	req := &EmbeddingBatchRequest{
		Prompts: prompts,
	}

	resp, err := c.client.GetEmbeddingBatch(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get batch embeddings: %w", err)
	}

	// Convert response to [][]float32
	embeddings := make([][]float32, len(resp.Embeddings))
	for i, emb := range resp.Embeddings {
		embeddings[i] = emb.Embedding
	}

	return embeddings, nil
}
