package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	// "github.com/tmc/langchaingo/llms/googleai"
)

func RunBaseline() {
	// Configuration
	embeddingServerAddr := "localhost:50051"

	// Load env (for GOOGLE_API_KEY etc)
	_ = godotenv.Load()

	// Read Pinecone config from env
	pineAPIKey := os.Getenv("PINECONE_API_KEY")
	pineIndex := os.Getenv("PINECONE_INDEX")
	if pineAPIKey == "" || pineIndex == "" {
		log.Println("Warning: PINECONE_API_KEY or PINECONE_INDEX not set in environment. Pinecone client may fail.")
	}

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal, shutting down gracefully...")
		cancel()
	}()

	// Initialize embedding client using existing helper in this package
	fmt.Printf("Connecting to embedding server at %s...\n", embeddingServerAddr)
	if err := InitEmbeddingClient(embeddingServerAddr); err != nil {
		log.Fatalf("Failed to initialize embedding client: %v", err)
	}
	defer CloseEmbeddingClient()
	fmt.Println("✓ Connected to embedding server")

	// Initialize Pinecone client (local implementation in this package)
	fmt.Println("\nInitializing Pinecone client...")
	pineClient, err := NewClient(pineAPIKey, pineIndex)
	if err != nil {
		log.Fatalf("Failed to initialize Pinecone client: %v", err)
	}
	fmt.Println("✓ Pinecone client initialized")

	// Define test queries - grouped by topic to see if similarity grouping works
	queries := []string{
		// Group 1: Machine Learning basics
		"Which events led to the outbreak of World War I?",
		"What were the major turning points that shifted momentum in World War II?",
		"How did the Cold War shape international relations during the 20th century?",

		"How did the printing press influence the spread of knowledge?",
		"What scientific advancements emerged from the Enlightenment era?",
		"How did the invention of the telegraph transform long-distance communication?",

		"What were the major causes of the French Revolution?",
		"How did the American Revolution impact global ideas about democracy?",
		"What factors led to the collapse of the Russian Empire in 1917?",

		"What themes are most prominent in Beyoncé’s album Lemonade?",
		"How did Taylor Swift’s Folklore reshape her artistic identity?",
		"What cultural movements influenced Kendrick Lamar’s album To Pimp a Butterfly?",

		"How did the Beatles influence popular music in the 1960s?",
		"What made Pink Floyd’s The Dark Side of the Moon a landmark concept album?",
		"How did Queen’s musical style evolve over the course of their career?",
	}

	line := strings.Repeat("=", 60)
	fmt.Printf("\n" + line + "\n")
	fmt.Printf("Embedding and querying %d queries...\n", len(queries))
	fmt.Printf(line + "\n\n")

	// Initialize LLM and call with the prompt template
	// llm, lerr := googleai.New(context.Background(), googleai.WithAPIKey(os.Getenv("GOOGLE_API_KEY")))
	// if lerr != nil {
	// 	// answers[idx] = fmt.Sprintf("ERROR initializing LLM: %v", lerr)
	// 	return
	// }

	// Process queries concurrently: embed each query using NewQuery (per-query embedding), then query Pinecone
	results := make([]*QueryResponse, len(queries))
	answers := make([]string, len(queries))
	errors := make([]error, len(queries))
	// durations: pinecone query times per query
	durations := make([]time.Duration, len(queries))
	// llmDurations: LLM call times per query
	llmDurations := make([]time.Duration, len(queries))
	var wg sync.WaitGroup

	totalStart := time.Now()
	for i, qText := range queries {
		wg.Add(1)
		go func(idx int, text string) {
			defer wg.Done()

			// Per-query context with timeout
			qCtx, qCancel := context.WithTimeout(ctx, 15*time.Second)
			defer qCancel()

			// Create query and compute embedding using helper (matches original pattern)
			qq, err := NewQuery(qCtx, text)
			if err != nil {
				errors[idx] = fmt.Errorf("failed to embed query: %w", err)
				return
			}

			// Query Pinecone with the computed embedding
			start := time.Now()
			resp, qerr := pineClient.Query(qCtx, QueryRequest{
				Vector:          qq.Embedding,
				TopK:            5,
				IncludeMetadata: true,
				IncludeValues:   false,
			})
			durations[idx] = time.Since(start)
			results[idx] = resp
			errors[idx] = qerr

			// Build context from Pinecone matches (use metadata["text"] when available)
			// var contextBuilder strings.Builder
			// if resp != nil {
			// 	for _, m := range resp.Matches {
			// 		if txt, ok := m.Metadata["text"].(string); ok {
			// 			contextBuilder.WriteString(txt)
			// 			contextBuilder.WriteString("\n\n")
			// 		}
			// 	}
			// }
			// contextText := contextBuilder.String()

			// prompt, perr := promptTemplate.Format(map[string]any{
			// 	"question": text,
			// 	"context":  contextText,
			// })
			// if perr != nil {
			// 	answers[idx] = fmt.Sprintf("ERROR building prompt: %v", perr)
			// 	return
			// }

			// // time the LLM call
			// llmStart := time.Now()
			// answer, aerr := llm.Call(qCtx, prompt)
			// llmDur := time.Since(llmStart)
			// llmDurations[idx] = llmDur
			// if aerr != nil {
			// 	answers[idx] = fmt.Sprintf("ERROR calling LLM: %v", aerr)
			// 	return
			// }
			// answers[idx] = answer
			answers[idx] = "Placeholder"
		}(i, qText)
	}

	wg.Wait()
	totalDuration := time.Since(totalStart)

	// Print results
	fmt.Println("\n" + line)
	fmt.Println("RESULTS")
	fmt.Println(line)
	successCount := 0
	for i, res := range results {
		if errors[i] != nil {
			fmt.Printf("\n[%d] ✗ FAILED\n", i+1)
			fmt.Printf("Query: %s\n", queries[i])
			fmt.Printf("Error: %v\n", errors[i])
			continue
		}
		successCount++
		fmt.Printf("\n[%d] ✓ SUCCESS\n", i+1)
		fmt.Printf("Query: %s\n", queries[i])
		if res == nil || len(res.Matches) == 0 {
			fmt.Printf("No matches returned\n")
			// Still print any LLM answer
			if answers[i] != "" {
				fmt.Printf("Answer: %s\n", answers[i])
			}
			continue
		}
		// fmt.Printf("Top %d matches:\n", len(res.Matches))
		// for j, m := range res.Matches {
		// 	fmt.Printf("  %d) ID=%s Score=%.4f Metadata=%+v\n", j+1, m.ID, m.Score, m.Metadata)
		// }
		fmt.Printf("Query time: %v\n", durations[i])
		// Print LLM answer based on context
		if answers[i] != "" {
			fmt.Printf("Answer: %s\n", answers[i])
		}
	}

	// Sum pinecone and LLM times
	var totalPine time.Duration
	var totalLLM time.Duration
	for i := range queries {
		totalPine += durations[i]
		totalLLM += llmDurations[i]
	}

	totalPine = totalPine / time.Duration(len(queries))
	totalLLM = totalLLM / time.Duration(len(queries))

	// Print summary and timings
	fmt.Println("\n" + line)
	fmt.Println("SUMMARY")
	fmt.Println(line)
	fmt.Printf("Total queries executed: %d\n", len(queries))
	fmt.Printf("Successful queries: %d\n", successCount)
	fmt.Printf("Failed queries: %d\n", len(queries)-successCount)
	fmt.Printf("Total time (all queries): %v\n", totalDuration)
	fmt.Printf("Total Pinecone query time (average): %v\n", totalPine)
	fmt.Printf("Total LLM call time (average): %v\n", totalLLM)
	avg := time.Duration(0)
	if len(queries) > 0 {
		avg = totalDuration / time.Duration(len(queries))
	}
	fmt.Printf("Average time per query: %v\n", avg)
	fmt.Println(line)
}
