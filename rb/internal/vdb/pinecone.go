package vdb

import (
	"context"
	"fmt"
	"log"

	"github.com/pinecone-io/go-pinecone/v4/pinecone"
	"google.golang.org/protobuf/types/known/structpb"
)

// Client wraps the Pinecone client
type Client struct {
	pineconeClient *pinecone.Client
	indexConn      *pinecone.IndexConnection
	indexName      string
}

// NewClient creates a new Pinecone client
func NewClient(apiKey, indexName string) (*Client, error) {
	ctx := context.Background()
	pc, err := pinecone.NewClient(pinecone.NewClientParams{
		ApiKey: apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Pinecone client: %w", err)
	}

	idx, err := pc.DescribeIndex(ctx, indexName)
	if err != nil {
		log.Fatalf("Failed to describe index \"%v\": %v", idx.Name, err)
	}

	indexConn, err := pc.Index(pinecone.NewIndexConnParams{Host: idx.Host, Namespace: ""})
	if err != nil {
		return nil, fmt.Errorf("failed to get index %s: %w", indexName, err)
	}

	return &Client{
		pineconeClient: pc,
		indexConn:      indexConn,
		indexName:      indexName,
	}, nil
}

// Upsert inserts or updates vectors in the index
func (c *Client) Upsert(ctx context.Context, vectors []Vector) error {
	// Convert our Vector type to Pinecone's Vector type
	pineconeVectors := make([]*pinecone.Vector, len(vectors))

	for i, v := range vectors {
		var metadataStruct *structpb.Struct
		var err error

		if v.Metadata != nil && len(v.Metadata) > 0 {
			metadataStruct, err = structpb.NewStruct(v.Metadata)
			if err != nil {
				return fmt.Errorf("failed to convert metadata for vector %s: %w", v.ID, err)
			}
		}

		pineconeVectors[i] = &pinecone.Vector{
			Id:       v.ID,
			Values:   &v.Values,
			Metadata: metadataStruct,
		}
	}

	_, err := c.indexConn.UpsertVectors(ctx, pineconeVectors)
	if err != nil {
		return fmt.Errorf("failed to upsert vectors: %w", err)
	}

	return nil
}

// Query searches for similar vectors
func (c *Client) Query(ctx context.Context, req QueryRequest) (*QueryResponse, error) {
	queryReq := pinecone.QueryRequest{
		Vector:          req.Vector,
		TopK:            req.TopK,
		IncludeMetadata: req.IncludeMetadata,
		IncludeValues:   req.IncludeValues,
	}

	// Add filter if provided
	if req.Filter != nil {
		// Convert filter to Pinecone's filter format
		filter := make(map[string]interface{})
		for k, v := range req.Filter {
			filter[k] = v
		}
		queryReq.Filter = filter
	}

	// Add namespace if provided
	if req.Namespace != "" {
		queryReq.Namespace = req.Namespace
	}

	resp, err := c.pineconeClient.Query(ctx, c.indexName, queryReq)
	if err != nil {
		return nil, fmt.Errorf("failed to query vectors: %w", err)
	}

	// Convert Pinecone response to our QueryResponse type
	queryResponse := &QueryResponse{
		Matches: make([]QueryResult, len(resp.Matches)),
	}

	for i, match := range resp.Matches {
		// Convert metadata back from map[string]string to map[string]interface{}
		metadata := make(map[string]interface{})
		for k, v := range match.Metadata {
			metadata[k] = v
		}

		queryResponse.Matches[i] = QueryResult{
			ID:       match.ID,
			Score:    match.Score,
			Values:   match.Values,
			Metadata: metadata,
		}
	}

	return queryResponse, nil
}

// Delete deletes vectors by IDs
func (c *Client) Delete(ctx context.Context, ids []string) error {
	_, err := c.pineconeClient.Delete(ctx, c.indexName, pinecone.DeleteRequest{
		IDs: ids,
	})
	if err != nil {
		return fmt.Errorf("failed to delete vectors: %w", err)
	}

	return nil
}

// DeleteByFilter deletes vectors matching a metadata filter
func (c *Client) DeleteByFilter(ctx context.Context, filter map[string]interface{}) error {
	pineconeFilter := make(map[string]interface{})
	for k, v := range filter {
		pineconeFilter[k] = v
	}

	_, err := c.pineconeClient.Delete(ctx, c.indexName, pinecone.DeleteRequest{
		Filter: pineconeFilter,
	})
	if err != nil {
		return fmt.Errorf("failed to delete vectors by filter: %w", err)
	}

	return nil
}

// GetIndexStats returns statistics about the index
func (c *Client) GetIndexStats(ctx context.Context) (map[string]interface{}, error) {
	stats, err := c.pineconeClient.DescribeIndexStats(ctx, c.indexName)
	if err != nil {
		return nil, fmt.Errorf("failed to get index stats: %w", err)
	}

	result := map[string]interface{}{
		"total_vector_count": stats.TotalVectorCount,
		"dimension":          stats.Dimension,
		"index_fullness":     stats.IndexFullness,
	}

	return result, nil
}
