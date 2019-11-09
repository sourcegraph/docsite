package index

import (
	"context"
)

// DocID is a unique identifier of a document in an index.
type DocID string

// Document is a document to be indexed.
type Document struct {
	ID   DocID  // the document ID
	Data []byte // the text content
}

// Index is a search index.
type Index struct {
	index map[DocID][]byte
}

// New returns a new index.
func New() (*Index, error) {
	return &Index{}, nil
}

// Add adds a document to the index.
func (i *Index) Add(ctx context.Context, doc Document) error {
	if i.index == nil {
		i.index = map[DocID][]byte{}
	}
	i.index[doc.ID] = doc.Data
	return nil
}
