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
		"tokenization": {
			text:  "aa bb cc",
			query: "cc bb",
			want:  []Match{{3, 5}, {6, 8}},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			q := Parse(test.query)
			matches := q.FindAllIndex([]byte(test.text))
			if !reflect.DeepEqual(matches, test.want) {
				t.Errorf("got matches %v, want %v", matches, test.want)
			}
		})
	}
}
