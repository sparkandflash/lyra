package contextManager

import (
	"context"
	"fmt"
	"msrpengine/src/utils"
	"os"
	"path/filepath"

	"github.com/philippgille/chromem-go"
)

// SearchResult represents a single context chunk returned from the vector database.
type SearchResult struct {
	ID       string
	Document string
	Score    float32
}

// IndexManager defines the interface for interacting with the vector database.
type IndexManager interface {
	AddDocument(collectionName string, id string, text string, metadata map[string]string) error
	SearchContext(collectionName string, query string, limit int) ([]SearchResult, error)
}

// ChromemIndexManager implements IndexManager using the chromem-go library.
type ChromemIndexManager struct {
	db *chromem.DB
}

// NewChromemIndexManager initializes a new local chromem-go persistent database.
// It ensures the Context/vector directory exists.
func NewChromemIndexManager() (*ChromemIndexManager, error) {
	baseDir := utils.ResolvePath(filepath.Join("Context", "vector"))
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create vector directory: %w", err)
	}

	// Initialize persistent chromem-go database
	db, err := chromem.NewPersistentDB(baseDir, false)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize chromem db: %w", err)
	}

	return &ChromemIndexManager{
		db: db,
	}, nil
}

// AddDocument generates an embedding for the given text and adds it to the specified collection.
func (m *ChromemIndexManager) AddDocument(collectionName string, id string, text string, metadata map[string]string) error {
	ctx := context.Background()

	// Get or create collection. Using nil for metadata and embedding function uses defaults.
	collection, err := m.db.GetOrCreateCollection(collectionName, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to get/create collection: %w", err)
	}

	// Create the chromem document
	doc := chromem.Document{
		ID:       id,
		Metadata: metadata,
		Content:  text,
	}

	// Add document to collection (generates embedding automatically)
	err = collection.AddDocuments(ctx, []chromem.Document{doc}, 1)
	if err != nil {
		return fmt.Errorf("failed to add document: %w", err)
	}

	return nil
}

// SearchContext performs a semantic search on the vector database and returns ranked results.
func (m *ChromemIndexManager) SearchContext(collectionName string, query string, limit int) ([]SearchResult, error) {
	ctx := context.Background()

	collection := m.db.GetCollection(collectionName, nil)
	if collection == nil {
		// If collection doesn't exist, simply return empty results
		return []SearchResult{}, nil
	}

	// Perform the query
	res, err := collection.Query(ctx, query, limit, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query collection: %w", err)
	}

	// Map results to our custom struct
	var results []SearchResult
	for _, r := range res {
		results = append(results, SearchResult{
			ID:       r.ID,
			Document: r.Content,
			Score:    r.Similarity,
		})
	}

	return results, nil
}

// IsMessageConsolidated checks the vector database for a document in the consolidation_index
// to determine if a message has already been processed by the consolidation loop.
func (m *ChromemIndexManager) IsMessageConsolidated(id string) bool {
	ctx := context.Background()
	collection := m.db.GetCollection("consolidation_index", nil)
	if collection == nil {
		return false
	}
	
	_, err := collection.GetByID(ctx, id)
	return err == nil
}

// MarkConsolidated adds the given message IDs to the consolidation_index without embedding them.
func (m *ChromemIndexManager) MarkConsolidated(ids []string) error {
	ctx := context.Background()
	
	collection, err := m.db.GetOrCreateCollection("consolidation_index", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to get/create consolidation index: %w", err)
	}
	
	docs := make([]chromem.Document, len(ids))
	for i, id := range ids {
		// Supplying a dummy embedding skips the actual embedding API call, making this instant and offline.
		docs[i] = chromem.Document{
			ID:        id,
			Content:   "true",
			Embedding: []float32{1.0},
		}
	}
	
	// Use high concurrency since it's just local memory operations without an API call
	err = collection.AddDocuments(ctx, docs, 10)
	if err != nil {
		return fmt.Errorf("failed to mark messages as consolidated: %w", err)
	}
	
	return nil
}
