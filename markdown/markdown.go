package markdown

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"

	chromahtml "github.com/alecthomas/chroma/formatters/html"
	"github.com/pkg/errors"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Document is a parsed and HTML-rendered Markdown document.
type Document struct {
	// Meta is the document's metadata in the Markdown "front matter", if any.
	Meta Metadata

	// Title is taken from the metadata (if it exists) or else from the text content of the first
	// heading.
	Title string

	// HTML is the rendered Markdown content.
	HTML []byte

	// Tree is the tree of sections (used to show a table of contents).
	Tree []*SectionNode
}

// Options customize how Run parses and HTML-renders the Markdown document.
type Options struct {
	// Base is the base URL (typically including only the path, such as "/" or "/help/") to use when
	// resolving relative links.
	Base *url.URL

	// ContentFilePathToLinkPath converts references to file paths of other content files to the URL
	// path to use in links. For example, ContentFilePathToLinkPath("a/index.md") == "a".
	ContentFilePathToLinkPath func(string) string

	// Funcs are custom functions that can be invoked within Markdown documents with
	// <markdownfuncjfunction-name arg="val" />.
	Funcs FuncMap

	// FuncInfo contains information passed to Markdown functions about the current execution
	// context.
	FuncInfo FuncInfo
}

// FuncMap contains named functions that can be invoked within Markdown documents (see
// (Options).Funcs).
type FuncMap map[string]func(context.Context, FuncInfo, map[string]string) (string, error)

// FuncInfo contains information passed to Markdown functions about the current execution context.
type FuncInfo struct {
	Version string // the version of the content containing the page to render
}

func New(opts Options) goldmark.Markdown {
	return goldmark.New(
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithAttribute(),
		),
		goldmark.WithRendererOptions(
			goldmarkhtml.WithXHTML(),
			goldmarkhtml.WithUnsafe(),
		),
		goldmark.WithExtensions(
			&extender{Options: opts},
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("github"),
				highlighting.WithFormatOptions(
					chromahtml.WithClasses(true),
					chromahtml.PreventSurroundingPre(true),
				),
				highlighting.WithWrapperRenderer(func(w util.BufWriter, context highlighting.CodeBlockContext, entering bool) {
					if entering {
						language, _ := context.Language()
						if len(language) == 0 {
							language = []byte("plaintext")
						}
						_, _ = w.WriteString(fmt.Sprintf(`<pre class="chroma %s">`, language))
					} else {
						_, _ = w.WriteString(`</pre>`)
					}
				}),
			),
		),
	)
}

// Run parses and HTML-renders a Markdown document (with optional metadata in the Markdown "front
// matter").
func Run(source []byte, opts Options) (doc *Document, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = errors.Errorf("panic while rendering Markdown: %s", e)
		}
	}()

	meta, source, err := parseMetadata(source)
	if err != nil {
		return nil, errors.Wrap(err, "parse metadata")
	}

	md := New(opts)

	ctx := parser.NewContext(
		func(cfg *parser.ContextConfig) {
			cfg.IDs = setHeadingIDs()
		},
	)
	mdAST := md.Parser().Parse(text.NewReader(source), parser.WithContext(ctx))
	tree := newTree(mdAST, source)
	title := meta.Title
	if title == "" {
		title = GetTitle(mdAST, source)
	}

	var buf bytes.Buffer
	err = md.Renderer().Render(&buf, source, mdAST)
	if err != nil {
		return nil, errors.Wrap(err, "render")
	}

	return &Document{
		Meta:  meta,
		Title: title,
		HTML:  buf.Bytes(),
		Tree:  tree,
	}, nil
}

var datePattern = regexp.MustCompile(
	// For examples, see:
	// https://developer.mozilla.org/en-US/docs/Web/HTML/Element/time#valid_datetime_values
	// https://developer.mozilla.org/en-US/docs/Web/HTML/Date_and_time_formats
	`\b(` +
		// Month string: MM-DD
		`[0-1][1-9]-[0-3][1-9]|` +
		// Week string: YYYY-WXX
		`\d{4}-W\d?\d|` +
		// Date or datetime string, with optional timezone: YYYY-MM-DD HH:MM:SS+HH:MM
		`\d{4}-\d{2}(-\d{2}([ T]?[0-2]?\d:\d\d(:\d\d(\.\d+)?)?(Z|[+-]\d\d:?\d\d)?)?)?|` +
		// Time string, with optional timezone: HH:MM:SS+HH:MM+HH:MM
		`(T?[0-2]?\d:\d\d(:\d\d(\.\d+)?)?(Z|[+-]\d\d:?\d\d)?)|` +
		// Fiscal year with optional quarter
		`FY[' -]?(\d{2,4})(?:[' -]?F?Q([1-4]))?|` +
		// Fiscal quarter only
		`F?Q([1-4])` +
		`)\b`)

var fiscalIntervalPattern = regexp.MustCompile(
	`^(?:` +
		// Fiscal year
		`FY[' -]?(\d{2,4})(?:[' -]?F?Q([1-4]))?|` +
		// Fiscal quarter only
		`F?Q([1-4])` +
		`)$`)

// parseFiscalInterval returns a YYYY-MM-DD or MM-DD date string for a reference to a fiscal quarter or fiscal year,
// for the start of the fiscal quarter or fiscal year, respectively.
func parseFiscalInterval(dateStr string) string {
	matches := fiscalIntervalPattern.FindStringSubmatch(dateStr)
	if len(matches) == 0 {
		return ""
	}
	fyStr := matches[1]
	qStr := matches[2]
	if fyStr == "" {
		qStr = matches[3]
	}

	fq := 1
	if qStr != "" {
		var err error
		fq, err = strconv.Atoi(qStr)
		if err != nil {
			panic(err) // regex pattern guarantees qStr is a digit
		}
	}

	// Start month of the quarter.
	// The fiscal year starts on February 1st.
	month := 2 + (fq-1)*3

	if fyStr == "" {
		// return MM-DD string
		return fmt.Sprintf("%02d-01", month)
	}

	// return YYYY-MM-DD string
	if len(fyStr) == 2 {
		fyStr = "20" + fyStr
	}
	fy, err := strconv.Atoi(fyStr)
	if err != nil {
		panic(err) // regex pattern guarantees fyStr contains only digits
	}
	year := fy - 1
	return fmt.Sprintf("%d-%02d-01", year, month)
}

// IsDocumentTopTitleHeadingNode reports whether node is an h1-heading at the top of the document.
func IsDocumentTopTitleHeadingNode(node ast.Node) bool {
	if node.Parent() != nil && node.Parent().Kind() == ast.KindDocument {
		return getDocumentTopTitleHeadingNode(node.Parent()) == node
	}
	return false
}

func getDocumentTopTitleHeadingNode(doc ast.Node) ast.Node {
	if doc.Kind() != ast.KindDocument {
		panic(fmt.Sprintf("got node type %q, want %q", doc.Kind(), ast.KindDocument))
	}

	for node := doc.FirstChild(); node != nil; node = node.NextSibling() {
		if node.Kind() != ast.KindHeading {
			continue
		}

		n := node.(*ast.Heading)
		if n.Level == 1 {
			return n
		}
		return nil
	}
	return nil
}

func GetTitle(doc ast.Node, source []byte) string {
	if doc.Kind() != ast.KindDocument {
		panic(fmt.Sprintf("got node type %q, want %q", doc.Kind(), ast.KindDocument))
	}

	title := getDocumentTopTitleHeadingNode(doc)
	if title != nil {
		return string(title.Text(source))
	}

	return ""
}
