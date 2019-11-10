package index

import (
	"context"
	"sort"

	"github.com/sourcegraph/docsite/internal/search/query"
)

// Result is the result of a search.
type Result struct {
	DocumentResults []DocumentResult // document results
	Total           int              // total number of document results
}

// DocumentResult is the result of a search for a single document.
type DocumentResult struct {
	Document
	Score float64
}

// Search performs a search against the index.
func (i *Index) Search(ctx context.Context, query query.Query) (*Result, error) {
	var documentResults []DocumentResult
	for _, doc := range i.index {
		if query.Match(doc.Data) {
			documentResults = append(documentResults, DocumentResult{
				Document: doc,
				Score:    query.Score(doc.Data),
			})
		}
	}
	sort.Slice(documentResults, func(i, j int) bool {
		return documentResults[i].Score > documentResults[j].Score || (documentResults[i].Score == documentResults[j].Score && documentResults[i].ID < documentResults[j].ID)
	})

	result := &Result{
		DocumentResults: documentResults,
		Total:           len(documentResults),
	}
	return result, nil
}
