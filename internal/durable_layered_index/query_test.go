package durableindex

import (
	"context"
	"testing"
	"time"
)

func TestNewQuery(t *testing.T) {
	ctx := context.Background()
	text := "test query"

	query := NewQuery(ctx, text)

	if query == nil {
		t.Fatal("NewQuery returned nil")
	}

	if query.Text != text {
		t.Errorf("expected text %q, got %q", text, query.Text)
	}

	if query.VectorEmbedding == nil {
		t.Fatal("VectorEmbedding is nil")
	}

	if len(query.VectorEmbedding) != 384 {
		t.Errorf("expected embedding length 384, got %d", len(query.VectorEmbedding))
	}

	if query.ResultChan == nil {
		t.Fatal("ResultChan is nil")
	}

	if cap(query.ResultChan) != 1 {
		t.Errorf("expected ResultChan capacity 1, got %d", cap(query.ResultChan))
	}

	if query.ctx != ctx {
		t.Error("context not set correctly")
	}
}

func TestEmbedText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected float32
	}{
		{
			name:     "empty string",
			text:     "",
			expected: 0,
		},
		{
			name:     "short text",
			text:     "hello",
			expected: 5,
		},
		{
			name:     "longer text",
			text:     "this is a test query",
			expected: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			embedding := EmbedText(tt.text)

			if len(embedding) != 384 {
				t.Errorf("expected embedding length 384, got %d", len(embedding))
			}

			for i, val := range embedding {
				if val != tt.expected {
					t.Errorf("at index %d: expected %f, got %f", i, tt.expected, val)
					break
				}
			}
		})
	}
}

func TestQuery_GetText(t *testing.T) {
	ctx := context.Background()
	text := "sample query text"

	query := NewQuery(ctx, text)

	if got := query.GetText(); got != text {
		t.Errorf("GetText() = %q, want %q", got, text)
	}
}

func TestQuery_GetEmbedding(t *testing.T) {
	ctx := context.Background()
	text := "test"

	query := NewQuery(ctx, text)
	embedding := query.GetEmbedding()

	if embedding == nil {
		t.Fatal("GetEmbedding() returned nil")
	}

	if len(embedding) != 384 {
		t.Errorf("expected embedding length 384, got %d", len(embedding))
	}

	// Verify the embedding is the same as the stored one
	for i := range embedding {
		if embedding[i] != query.VectorEmbedding[i] {
			t.Errorf("embedding mismatch at index %d", i)
			break
		}
	}
}

func TestQuery_GetContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	query := NewQuery(ctx, "test")

	if got := query.GetContext(); got != ctx {
		t.Error("GetContext() did not return the correct context")
	}
}

func TestQuery_SetResult(t *testing.T) {
	ctx := context.Background()
	query := NewQuery(ctx, "test")

	expectedResult := "test result"
	query.SetResult(expectedResult)

	select {
	case result := <-query.ResultChan:
		if result != expectedResult {
			t.Errorf("expected result %q, got %q", expectedResult, result)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for result")
	}
}

func TestQuery_SetResult_ChannelFull(t *testing.T) {
	ctx := context.Background()
	query := NewQuery(ctx, "test")

	// Fill the channel
	query.SetResult("first result")

	// This should not block even though channel is full
	done := make(chan bool)
	go func() {
		query.SetResult("second result")
		done <- true
	}()

	select {
	case <-done:
		// Success - SetResult didn't block
	case <-time.After(100 * time.Millisecond):
		t.Error("SetResult blocked when channel was full")
	}

	// Verify only the first result is in the channel
	result := <-query.ResultChan
	if result != "first result" {
		t.Errorf("expected first result, got %v", result)
	}

	// Channel should now be empty
	select {
	case <-query.ResultChan:
		t.Error("unexpected value in channel")
	default:
		// Expected - channel is empty
	}
}

func TestQuery_WaitForResult_Success(t *testing.T) {
	ctx := context.Background()
	query := NewQuery(ctx, "test")

	expectedResult := "test result"

	// Send result in a goroutine
	go func() {
		time.Sleep(10 * time.Millisecond)
		query.SetResult(expectedResult)
	}()

	result, err := query.WaitForResult()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result != expectedResult {
		t.Errorf("expected result %q, got %q", expectedResult, result)
	}
}

func TestQuery_WaitForResult_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	query := NewQuery(ctx, "test")

	// Cancel context immediately
	cancel()

	result, err := query.WaitForResult()

	if err == nil {
		t.Error("expected error when context is cancelled")
	}

	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}

	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

func TestQuery_WaitForResult_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	query := NewQuery(ctx, "test")

	// Don't send any result, let it timeout

	result, err := query.WaitForResult()

	if err == nil {
		t.Error("expected error when context times out")
	}

	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded error, got %v", err)
	}

	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

func TestQuery_WaitForResult_RaceCondition(t *testing.T) {
	// Test for race conditions when result is set before waiting
	ctx := context.Background()
	query := NewQuery(ctx, "test")

	expectedResult := "immediate result"
	query.SetResult(expectedResult)

	// Wait should return immediately
	done := make(chan bool)
	go func() {
		result, err := query.WaitForResult()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != expectedResult {
			t.Errorf("expected result %q, got %q", expectedResult, result)
		}
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("WaitForResult did not return immediately when result was already set")
	}
}

func TestQuery_MultipleGoroutines(t *testing.T) {
	// Test concurrent access to query methods
	ctx := context.Background()
	query := NewQuery(ctx, "concurrent test")

	done := make(chan bool)

	// Multiple goroutines reading data
	for i := 0; i < 10; i++ {
		go func() {
			_ = query.GetText()
			_ = query.GetEmbedding()
			_ = query.GetContext()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// Success
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for goroutines")
		}
	}
}

func BenchmarkNewQuery(b *testing.B) {
	ctx := context.Background()
	text := "benchmark query text"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewQuery(ctx, text)
	}
}

func BenchmarkEmbedText(b *testing.B) {
	text := "benchmark text"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = EmbedText(text)
	}
}

func BenchmarkQuery_SetResult(b *testing.B) {
	ctx := context.Background()
	query := NewQuery(ctx, "test")
	result := "benchmark result"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Drain the channel before each set
		select {
		case <-query.ResultChan:
		default:
		}
		query.SetResult(result)
	}
}
