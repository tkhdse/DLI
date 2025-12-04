package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
    "github.com/tmc/langchaingo/llms/googleai"
    "github.com/tmc/langchaingo/embeddings"
    "github.com/tmc/langchaingo/prompts"
)

// Bin batches related queries together for efficient processing
type Bin struct {
	dbCollection interface{}
	queue        []*Query
	mu           sync.Mutex
	maxBatchSize int
	maxWaitTime  time.Duration
	flushCh      chan struct{}
	shutdownCh   chan struct{}
	wg           sync.WaitGroup
}

func initialize() string {
	godotenv.Load()
	return ""
}

var st = initialize()

var (
	queryEmbedder embeddings.Embedder
	promptTemplate = prompts.NewPromptTemplate(
		`Use ONLY the following context to answer the question.

		Question: {{.question}}

		Context:
		{{.context}}

		If the answer is not in the context, say "I cannot find the answer in the context."`,
		[]string{"question", "context"},
	)
	ctx = context.Background()
	llm, err = googleai.New(
		ctx,
		googleai.WithAPIKey(os.Getenv("GOOGLE_API_KEY")),
	)
	client, _ = NewClient(os.Getenv("PINECONE_API_KEY"), "durable-layered-index")
)




// NewBin creates a new Bin instance
func NewBin(dbCollection interface{}, maxBatchSize int, maxWaitTime time.Duration) *Bin {
	b := &Bin{
		dbCollection: dbCollection,
		queue:        make([]*Query, 0, maxBatchSize),
		maxBatchSize: maxBatchSize,
		maxWaitTime:  maxWaitTime,
		flushCh:      make(chan struct{}, 1),
		shutdownCh:   make(chan struct{}),
	}

	b.wg.Add(1)
	go b.batchLoop()

	return b
}

// SetBatchSize updates the maximum batch size
func (b *Bin) SetBatchSize(size int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	oldSize := b.maxBatchSize
	b.maxBatchSize = size

	// If current queue exceeds new size, trigger flush
	if len(b.queue) >= size && size < oldSize {
		b.triggerFlush()
	}
}

// SetWaitTime updates the maximum wait time
func (b *Bin) SetWaitTime(duration time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.maxWaitTime = duration
}

// AddQuery adds a query to the bin's queue
func (b *Bin) AddQuery(q *Query) {
	b.mu.Lock()
	b.queue = append(b.queue, q)
	shouldFlush := len(b.queue) >= b.maxBatchSize
	b.mu.Unlock()

	if shouldFlush {
		b.triggerFlush()
	}
}

// triggerFlush signals the batch loop to process immediately
func (b *Bin) triggerFlush() {
	select {
	case b.flushCh <- struct{}{}:
	default:
		// Channel already has a signal
	}
}

// Shutdown gracefully stops the bin
func (b *Bin) Shutdown(ctx context.Context) error {
	close(b.shutdownCh)

	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// batchLoop continuously processes batches
func (b *Bin) batchLoop() {
	defer b.wg.Done()

	timer := time.NewTimer(b.maxWaitTime)
	defer timer.Stop()

	for {
		select {
		case <-b.shutdownCh:
			b.processBatch()
			return

		case <-b.flushCh:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			b.processBatch()
			timer.Reset(b.maxWaitTime)

		case <-timer.C:
			b.processBatch()
			timer.Reset(b.maxWaitTime)
		}
	}
}

// processBatch processes all queued queries
func (b *Bin) processBatch() {
	b.mu.Lock()
	batch := b.queue
	b.queue = make([]*Query, 0, b.maxBatchSize)
	b.mu.Unlock()

	if len(batch) == 0 {
		return
	}

	texts := make([]string, len(batch))
	embeddings := make([][]float32, len(batch))

	for i, q := range batch {
		texts[i] = q.Text
		embeddings[i] = q.Embedding
	}

	results := b.batchVectorDBQuery(texts, embeddings)

	for i, q := range batch {
		q.SetResult(results[i])
	}
}

// batchVectorDBQuery performs the actual database query
func (b *Bin) batchVectorDBQuery(texts []string, embeddings [][]float32) []string {
	// TODO: Implement actual database query logic
	// For now, simulating with artificial delay
	time.Sleep(100 * time.Millisecond)
	cont, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results := make([]string, len(texts))

	
	for i, text := range texts {
		qr := QueryRequest{
			Vector:          embeddings[i],
			TopK:            5,
			IncludeValues:   true,
			IncludeMetadata: true,
		}
		var contextBuilder strings.Builder
		res, _ := client.Query(cont, qr)
		for _, m := range res.Matches {
			if txt, ok := m.Metadata["text"].(string); ok {
				contextBuilder.WriteString(txt)
				contextBuilder.WriteString("\n\n")
			}
		}
		contextText := contextBuilder.String()
		prompt, err := promptTemplate.Format(map[string]any{
			"question": text,
			"context":  contextText,
		})
		answer, err := llm.Call(ctx, prompt)
		if err != nil {
			fmt.Printf("LLM error: %v\n", err)
			results[i] = "ERROR calling Gemini"
			continue
		}
		results[i] = answer
	}

	return results
}
