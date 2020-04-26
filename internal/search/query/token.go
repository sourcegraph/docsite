package query

import "regexp"

type token struct {
	str     string
	pattern *regexp.Regexp
}

func newToken(str string) token {
	return token{
		str:     str,
		pattern: regexp.MustCompile("(?i)" + regexp.QuoteMeta(str)),
	}
}
