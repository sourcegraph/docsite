package markdown

import (
	"github.com/alecthomas/chroma"
	. "github.com/alecthomas/chroma" // nolint
	"github.com/alecthomas/chroma/lexers"
)

// Sourcegraph query lexer.
var SGQuery = lexers.Register(chroma.MustNewLexer(
	&Config{
		Name:            "Sourcegraph Query",
		Aliases:         []string{"sgquery", "query"},
		CaseInsensitive: true,
	},
	sgqueryRules(),
))

//nolint
func sgqueryRules() Rules {
	return Rules{
		"root": {
			{`\s+`, TextWhitespace, nil},
			{Words("-?", "", `repo:`, `r:`), NameBuiltin, Push("repo")},
			{Words("-?", "", `file:`, `f:`), NameBuiltin, Push("regexp")},
			{Words("-?", "",
				`case:`,
				`repogroup:`,
				`fork`,
				`archived`,
				`lang:`,
				`type:`,
				`repohasfile:`,
				`repohascommitafter:`,
				`patterntype:`,
				`content:`,
				`visibility:`,
				`rev:`,
				`context:`,
				`before:`,
				`after:`,
				`author:`,
				`committer:`,
				`message:`,
				`index:`,
				`count:`,
				`stable:`,
				`rule:`,
				`select:`,
			), NameBuiltin, Push("value")},
			{Words("", `\b`, `and`, `or`), Keyword, nil},
			{`\(`, Punctuation, Push()},
			{`\)`, Punctuation, Pop(1)},
			{`[^\s]+`, Text, nil},
		},
		"repo": {
			{`(contains)(\()`, ByGroups(NameFunction, Punctuation), Push("repoContains")},
			{`@`, Punctuation, Push("revs")},
			{`\s`, TextWhitespace, Pop(1)},
			Default(Push("regexp")),
		},
		"revs": {
			{`\*`, LiteralStringSymbol, Push("rev")},
			{`!\*`, LiteralStringSymbol, Push("rev")},
			{`(?=\w)`, Text, Push("rev")},
			Default(Pop(2)),
		},
		"rev": {
			{`\*`, LiteralStringAffix, nil},
			{`[-\w/^]+`, Text, nil},
			{`:`, LiteralStringDelimiter, Pop(1)},
			Default(Pop(2)),
		},
		"repoContains": {
			{`\)`, Punctuation, Pop(2)},
			{Words("", "", `file:`, `content:`), NameBuiltin, Push("regexp")},
			{``, Text, Push("regexp")},
		},
		"value": {
			{`"`, Text, Push("dqString")},
			{`'`, Text, Push("sqString")},
			{`\s`, Whitespace, Pop(1)},
			{`[^'"\s]*`, Text, nil},
		},
		"dqString": {
			{`"`, Text, Pop(2)},
			{`[^"]*`, Text, nil},
		},
		"sqString": {
			{`'`, Text, Pop(2)},
			{`[^']*`, Text, nil},
		},
		"subquery": {
			{`\)`, Punctuation, Pop(1)},
			Include("root"),
		},
		"regexp": {
			{`\\.`, LiteralStringEscape, nil},
			{`[\$\^\.\+\[\]\(\)]`, LiteralStringRegex, nil},
			{`[^$^\.\s\\]`, Text, nil},
			{`\s`, TextWhitespace, Pop(1)},
		},
	}
}
