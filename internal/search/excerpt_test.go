package search

import "testing"

func TestExcerpt(t *testing.T) {
	tests := map[string]struct {
		text       string
		start, end int
		maxChars   int
		want       string
	}{
		"expand both sides": {
			text:  "a b c",
			start: 2, end: 3,
			maxChars: 5,
			want:     "a b c",
		},
		"clip": {
			text:  "a",
			start: 0, end: 1,
			maxChars: 5,
			want:     "a",
		},
		"break on sentences": {
			text:  "hello world. foo bar. baz qux.",
			start: 13, end: 16,
			maxChars: 21,
			want:     "foo bar. baz qux.",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got := excerpt(test.text, test.start, test.end, test.maxChars)
			if got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
