package query

import (
	"reflect"
	"testing"
)

func TestQuery_FindAllIndex(t *testing.T) {
	tests := map[string]struct {
		text  string
		query string
		want  []Match
	}{
		"simple": {
			text:  "aa bb aa",
			query: "aa",
			want:  []Match{{0, 2}, {6, 8}},
		},
		"token substring of another token": {
			text:  "aa",
			query: "a aa",
			want:  []Match{{0, 1}, {0, 2}, {1, 2}},
		},
		"token substring of another token with only shorter match": {
			text:  "ab",
			query: "ab abc",
			want:  []Match{{0, 2}},
		},
		"duplicate tokens": {
			text:  "a",
			query: "a a",
			want:  []Match{{0, 1}},
		},
		"tokenization": {
			text:  "aa bb cc",
			query: "cc bb",
			want:  []Match{{3, 5}, {6, 8}},
		},
		"case-insensitive": {
			text:  "a A b B",
			query: "a B",
			want:  []Match{{0, 1}, {2, 3}, {4, 5}, {6, 7}},
		},
		"wide chars": {
			text:  "a \x9f\x92\xa1 a",
			query: "a",
			want:  []Match{{0, 1}, {6, 7}},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			q := Parse(test.query)
			matches := q.FindAllIndex(test.text)
			if !reflect.DeepEqual(matches, test.want) {
				t.Errorf("got matches %v, want %v", matches, test.want)
			}
		})
	}
}
