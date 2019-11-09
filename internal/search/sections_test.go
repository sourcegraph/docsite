package search

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/sourcegraph/docsite/internal/search/query"
)

func TestDocumentSectionResults(t *testing.T) {
	toResultsList := func(srs []SectionResult) []string {
		l := make([]string, len(srs))
		for i, sr := range srs {
			l[i] = fmt.Sprintf("#%s", sr.ID)
		}
		return l
	}

	tests := map[string]struct {
		data             string
		wantQueryResults map[string][]string
	}{
		"simple": {
			data: `# A
aa zz

## B

bb zz`,
			wantQueryResults: map[string][]string{
				"a":  []string{"#"},
				"aa": []string{"#"},
				"b":  []string{"#b"},
				"bb": []string{"#b"},
				"zz": []string{"#", "#b"},
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			for queryStr, wantResults := range test.wantQueryResults {
				t.Run(queryStr, func(t *testing.T) {
					results, err := documentSectionResults([]byte(test.data), query.Parse(queryStr))
					if err != nil {
						t.Fatal(err)
					}
					if gotResults := toResultsList(results); !reflect.DeepEqual(gotResults, wantResults) {
						t.Errorf("got results %v, want %v", gotResults, wantResults)
					}
				})
			}
		})
	}
}
