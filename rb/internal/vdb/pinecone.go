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
		log.Fatalf("Failed to describe index \"%v\": %v", indexName, err)
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

	// Add nil check for indexConn (defensive)
	if c.indexConn == nil {
		return fmt.Errorf("index connection is nil")
	}

	_, err := c.indexConn.UpsertVectors(ctx, pineconeVectors)
	if err != nil {
		return fmt.Errorf("failed to upsert vectors: %w", err)
	}

	return nil
}

// Query searches for similar vectors
func (c *Client) Query(ctx context.Context, req QueryRequest) (*QueryResponse, error) {

	queryReq := &pinecone.QueryByVectorValuesRequest{
		Vector:          req.Vector,
		TopK:            uint32(req.TopK),
		IncludeMetadata: req.IncludeMetadata,
		IncludeValues:   req.IncludeValues,
	}

	// Set up metadata filter if provided
	if req.Filter != nil && len(req.Filter) > 0 {
		metadataFilter, err := structpb.NewStruct(req.Filter)
		if err != nil {
			log.Fatalf("Failed to create metadataFilter: %v", err)
		}
		// Set the filter on the query request
		queryReq.MetadataFilter = metadataFilter
	}

	resp, err := c.indexConn.QueryByVectorValues(ctx, queryReq)
	if err != nil {
		return nil, fmt.Errorf("failed to query vectors: %w", err)
	}

	// Convert Pinecone response to our QueryResponse type
	queryResponse := &QueryResponse{
		Matches: make([]QueryResult, len(resp.Matches)),
	}

	for i, match := range resp.Matches {
		// Initialize defaults
		metadata := make(map[string]interface{})
		values := []float32{}
		id := ""

		// Safely access Vector fields
		if match.Vector != nil {
			id = match.Vector.Id

			// Handle Metadata - check for nil
			if match.Vector.Metadata != nil {
				metadata = match.Vector.Metadata.AsMap()
			}

			// Handle Values - check for nil pointer
			if match.Vector.Values != nil {
				values = *match.Vector.Values
			}
		}

		queryResponse.Matches[i] = QueryResult{
			ID:       id,
			Score:    match.Score,
			Values:   values,
			Metadata: metadata,
		}
	}

	return queryResponse, nil
}

// Delete deletes vectors by IDs
func (c *Client) Delete(ctx context.Context, ids []string) error {
	err := c.indexConn.DeleteVectorsById(ctx, ids)
	if err != nil {
		return fmt.Errorf("failed to delete vectors: %w", err)
	}

	return nil
}

// DeleteByFilter deletes vectors matching a metadata filter
func (c *Client) DeleteByFilter(ctx context.Context, filter map[string]interface{}) error {
	pineconeFilterStruct := make(map[string]interface{})
	for k, v := range filter {
		pineconeFilterStruct[k] = v
	}

	pineconeFilter, err := structpb.NewStruct(pineconeFilterStruct)
	if err != nil {
		log.Fatalf("Failed to create metadata filter. Error: %v", err)
	}

	err = c.indexConn.DeleteVectorsByFilter(ctx, pineconeFilter)
	if err != nil {
		return fmt.Errorf("failed to delete vectors by filter: %w", err)
	}

	return nil
}

// GetIndexStats returns statistics about the index
func (c *Client) GetIndexStats(ctx context.Context) (map[string]interface{}, error) {
	idx, err := c.pineconeClient.DescribeIndex(ctx, c.indexName)
	if err != nil {
		return nil, fmt.Errorf("failed to get index stats: %w", err)
	}

	// Defensive check (though idx should not be nil if err is nil)
	if idx == nil {
		return nil, fmt.Errorf("index description returned nil")
	}

	result := map[string]interface{}{
		"metric":    idx.Metric,
		"dimension": idx.Dimension,
		"status":    idx.Status,
	}

	return result, nil
}
