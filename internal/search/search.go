package search

import (
	"context"

	"github.com/pkg/errors"
	"github.com/sourcegraph/docsite/internal/search/index"
	"github.com/sourcegraph/docsite/internal/search/query"
)

// Result is the result of a search.
type Result struct {
	DocumentResults []DocumentResult // document results
	Total           int              // total number of document results
}

// DocumentResult is the result of a search for a single document
type DocumentResult struct {
	index.DocumentResult
	SectionResults []SectionResult
}

func Search(ctx context.Context, query query.Query, index *index.Index) (*Result, error) {
	result0, err := index.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	result := &Result{
		DocumentResults: make([]DocumentResult, len(result0.DocumentResults)),
		Total:           result0.Total,
	}
	for i, dr := range result0.DocumentResults {
		srs, err := documentSectionResults(dr.Data, query)
		if err != nil {
			return nil, errors.WithMessagef(err, "document section results for %q", dr.ID)
		}
		result.DocumentResults[i] = DocumentResult{
			DocumentResult: dr,
			SectionResults: srs,
		}
	}
	return result, nil
}
