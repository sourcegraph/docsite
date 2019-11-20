package query

import (
	"bytes"
	"path"
	"sort"
)

// Query is a search query.
type Query struct {
	input  string // the original query input string
	tokens [][]byte
}

// Parse parses a search query string.
func Parse(queryStr string) Query {
	return Query{
		input:  queryStr,
		tokens: bytes.Fields(bytes.ToLower([]byte(queryStr))),
	}
}

// Match reports whether b matches the query.
func (q Query) Match(pathStr string, b []byte) bool {
	name := []byte(path.Base(pathStr))

	b = bytes.ToLower(b)
	for _, token := range q.tokens {
		if bytes.Contains(name, token) {
			return true
		}
		if bytes.Contains(b, token) {
			return true
		}
	}
	return false
}

// Score scores the query match against b.
func (q Query) Score(pathStr string, b []byte) float64 {
	name := []byte(path.Base(pathStr))

	b = bytes.ToLower(b)
	tokensInName := 0
	tokensMatching := 0
	totalMatches := 0
	for _, token := range q.tokens {
		if bytes.Contains(name, token) {
			tokensInName++
		}
		count := bytes.Count(b, token)
		if count > 0 {
			tokensMatching++
		}
		totalMatches += count
	}

	return float64(tokensInName*500) + float64(tokensMatching*100) + float64(totalMatches)/float64(len(b)+1)
}

// Match is an array of [start, end] indexes for a match.
type Match [2]int

// FindAllIndex returns a slice of all query match indexes in b.
func (q Query) FindAllIndex(b []byte) []Match {
	b = bytes.ToLower(b)
	findTokenAllIndex := func(token []byte) []Match {
		var matches []Match
		c := 0
		for c < len(b) {
			start := bytes.Index(b[c:], token)
			if start == -1 {
				break
			}
			start += c
			end := start + len(token)
			matches = append(matches, Match{start, end})
			c = end
		}
		return matches
	}

	var matches []Match
	for _, token := range q.tokens {
		matches = append(matches, findTokenAllIndex(token)...)
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i][0] < matches[j][0] || (matches[i][0] == matches[j][0] && matches[i][1] < matches[j][1])
	})
	return matches
}
