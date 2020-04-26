package query

import (
	"path"
	"sort"
	"strings"
)

// Query is a search query.
type Query struct {
	input  string // the original query input string
	tokens []token
}

// Parse parses a search query string.
func Parse(queryStr string) Query {
	tokenStrs := strings.Fields(queryStr)
	tokens := make([]token, len(tokenStrs))
	for i, tokenStr := range tokenStrs {
		tokens[i] = newToken(tokenStr)
	}

	return Query{
		input:  queryStr,
		tokens: tokens,
	}
}

// Match reports whether the path or text contains at least 1 match of the query.
func (q Query) Match(pathStr, text string) bool {
	name := path.Base(pathStr)

	for _, token := range q.tokens {
		if token.pattern.MatchString(name) {
			return true
		}
		if token.pattern.MatchString(text) {
			return true
		}
	}
	return false
}

const maxMatchesPerDoc = 50

// Score scores the query match against the path and text.
func (q Query) Score(pathStr, text string) float64 {
	name := path.Base(pathStr)

	tokensInName := 0
	tokensMatching := 0
	totalMatches := 0
	for _, token := range q.tokens {
		if token.pattern.MatchString(name) {
			tokensInName++
		}
		count := len(token.pattern.FindAllStringIndex(text, maxMatchesPerDoc))
		if count > 0 {
			tokensMatching++
		}
		totalMatches += count
	}

	return float64(tokensInName*500) + float64(tokensMatching*100) + float64(totalMatches)/float64(len(text)+1)
}

// Match is an array of [start, end] byte indexes for a match.
type Match [2]int

// FindAllIndex returns a slice of all query match indexes in the text.
func (q Query) FindAllIndex(text string) []Match {
	findTokenAllIndex := func(token token) []Match {
		var matches []Match
		c := 0
		for c < len(text) {
			m := token.pattern.FindStringIndex(text[c:])
			if m == nil {
				break
			}
			start, end := c+m[0], c+m[1]
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
