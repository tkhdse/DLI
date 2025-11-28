package vdb

// Vector represents a vector with its ID, values, and metadata
type Vector struct {
	ID       string
	Values   []float32
	Metadata map[string]interface{}
}

// QueryRequest represents a query to find similar vectors
type QueryRequest struct {
	Vector          []float32
	TopK            int
	IncludeMetadata bool
	IncludeValues   bool
	Filter          map[string]interface{} // Optional metadata filter
	Namespace       string                 // Optional namespace
}

// QueryResult represents a single match from a query
type QueryResult struct {
	ID       string
	Score    float32
	Values   []float32
	Metadata map[string]interface{}
}

// QueryResponse represents the full response from a query
type QueryResponse struct {
	Matches []QueryResult
}
