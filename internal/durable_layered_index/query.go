package durableindex

import (
	"context"
)

// Query represents a search query with its embedding and result channel
type Query struct {
	Text            string
	VectorEmbedding []float32
	ResultChan      chan interface{}
	ctx             context.Context
}

// TODO: Figure out how embeddings work, inside NewQuery, we create the embedding
func NewQuery(ctx context.Context, text string) *Query {
	embedding := EmbedText(text)
	return &Query{
		Text:            text,
		VectorEmbedding: embedding,
		ResultChan:      make(chan interface{}, 1), // buffered to avoid blocking
		ctx:             ctx,
	}
}

func EmbedText(text string) []float32 {
	vec := make([]float32, 384)
	for i := range vec {
		vec[i] = float32(len(text))
	}
	return vec
}

// GetText returns the query text
func (q *Query) GetText() string {
	return q.Text
}

// GetEmbedding returns the vector embedding
func (q *Query) GetEmbedding() []float32 {
	return q.VectorEmbedding
}

// GetContext returns the query context
func (q *Query) GetContext() context.Context {
	return q.ctx
}

// SetResult sends the result to the result channel
func (q *Query) SetResult(result interface{}) {
	select {
	case q.ResultChan <- result:
	default:
		// Channel already has a result or is closed
	}
}

// WaitForResult blocks until a result is available or context is cancelled
func (q *Query) WaitForResult() (interface{}, error) {
	select {
	case result := <-q.ResultChan:
		return result, nil
	case <-q.ctx.Done():
		return nil, q.ctx.Err()
	}
}
