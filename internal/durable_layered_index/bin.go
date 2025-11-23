package durableindex

import (
	"fmt"
)

package index

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DBCollection interface for database operations
type DBCollection interface {
	BatchQuery(ctx context.Context, texts []string, embeddings [][]float32) ([]interface{}, error)
}

// Bin handles batching of queries for efficient database access
type Bin struct {
	dbCollection   DBCollection
	queue          []*Query
	mu             sync.Mutex
	flushChan      chan struct{}
	shutdownChan   chan struct{}
	wg             sync.WaitGroup
	maxBatchSize   int
	maxWaitTime    time.Duration
	batchSizeMu    sync.RWMutex
	shutdown       bool
}

// NewBin creates a new Bin with the specified parameters
func NewBin(dbCollection DBCollection, maxBatchSize int, maxWaitTime time.Duration) *Bin {
	b := &Bin{
		dbCollection: dbCollection,
		queue:        make([]*Query, 0, maxBatchSize),
		flushChan:    make(chan struct{}, 1),
		shutdownChan: make(chan struct{}),
		maxBatchSize: maxBatchSize,
		maxWaitTime:  maxWaitTime,
	}

	b.wg.Add(1)
	go b.batchLoop()

	return b
}

// SetBatchSize updates the maximum batch size
func (b *Bin) SetBatchSize(batchSize int) {
	b.batchSizeMu.Lock()
	b.maxBatchSize = batchSize
	b.batchSizeMu.Unlock()

	// TODO: Force flush if current queue exceeds new batch size
	b.mu.Lock()
	shouldFlush := len(b.queue) >= batchSize
	b.mu.Unlock()

	if shouldFlush {
		b.signalFlush()
	}
}

// SetWaitTime updates the maximum wait time
func (b *Bin) SetWaitTime(waitTime time.Duration) {
	b.batchSizeMu.Lock()
	b.maxWaitTime = waitTime
	b.batchSizeMu.Unlock()

	// Signal to restart timer with new duration
	b.signalFlush()
}

// Shutdown gracefully shuts down the bin, processing remaining queries
func (b *Bin) Shutdown(ctx context.Context) error {
	b.mu.Lock()
	if b.shutdown {
		b.mu.Unlock()
		return nil
	}
	b.shutdown = true
	b.mu.Unlock()

	close(b.shutdownChan)

	// Wait for batch loop to finish with timeout
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

// AddQuery adds a query to the bin for batch processing
func (b *Bin) AddQuery(query *Query) error {
	b.mu.Lock()
	if b.shutdown {
		b.mu.Unlock()
		return fmt.Errorf("bin is shut down")
	}

	b.queue = append(b.queue, query)
	queueSize := len(b.queue)
	b.mu.Unlock()

	b.batchSizeMu.RLock()
	maxBatch := b.maxBatchSize
	b.batchSizeMu.RUnlock()

	// Trigger immediate flush if we've reached max batch size
	if queueSize >= maxBatch {
		b.signalFlush()
	}

	return nil
}

// signalFlush sends a non-blocking signal to process the batch
func (b *Bin) signalFlush() {
	select {
	case b.flushChan <- struct{}{}:
	default:
		// Already has a pending flush signal
	}
}

// batchLoop is the main goroutine that processes batches
func (b *Bin) batchLoop() {
	defer b.wg.Done()

	b.batchSizeMu.RLock()
	timer := time.NewTimer(b.maxWaitTime)
	b.batchSizeMu.RUnlock()

	defer timer.Stop()

	for {
		select {
		case <-b.shutdownChan:
			// Process any remaining queries before shutting down
			b.processBatch(context.Background())
			return

		case <-b.flushChan:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			b.processBatch(context.Background())

			b.batchSizeMu.RLock()
			timer.Reset(b.maxWaitTime)
			b.batchSizeMu.RUnlock()

		case <-timer.C:
			b.processBatch(context.Background())

			b.batchSizeMu.RLock()
			timer.Reset(b.maxWaitTime)
			b.batchSizeMu.RUnlock()
		}
	}
}

// processBatch processes the current batch of queries
func (b *Bin) processBatch(ctx context.Context) {
	b.mu.Lock()
	if len(b.queue) == 0 {
		b.mu.Unlock()
		return
	}

	// Take the current batch and reset the queue
	batch := b.queue
	b.queue = make([]*Query, 0, b.maxBatchSize)
	b.mu.Unlock()

	// Prepare batch data
	texts := make([]string, len(batch))
	embeddings := make([][]float32, len(batch))

	for i, query := range batch {
		texts[i] = query.GetText()
		embeddings[i] = query.GetEmbedding()
	}

	// Execute batch query
	results, err := b.dbCollection.BatchQuery(ctx, texts, embeddings)

	// Send results back to each query
	for i, query := range batch {
		if err != nil {
			query.SetResult(err)
		} else if i < len(results) {
			query.SetResult(results[i])
		} else {
			query.SetResult(fmt.Errorf("no result returned for query"))
		}
	}
}