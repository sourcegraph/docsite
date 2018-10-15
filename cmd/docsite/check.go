package main

import (
	"bytes"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func init() {
	flagSet := flag.NewFlagSet("check", flag.ExitOnError)
	var (
		// TODO(sqs): do not skip "^#" URLs; instead, check them.
		skipURLs = flagSet.String("skip-urls", "", "regexp `pattern` for URLs to skip in broken link check")
	)

	handler := func(args []string) error {
		flagSet.Parse(args)

		var problems []string

		var skipURLsPattern *regexp.Regexp
		if *skipURLs != "" {
			var err error
			skipURLsPattern, err = regexp.Compile(*skipURLs)
			if err != nil {
				return err
			}
		}
		skipURL := func(url string) bool {
			return skipURLsPattern != nil && skipURLsPattern.MatchString(url)
		}

		// Check .html files.
		gen := generatorFromFlags()
		handler := newHandler()
		checkFile := func(path string) error {
			if skipFile(path) {
				return nil
			}
			data, err := gen.Generate(path, true)
			if err != nil {
				return err
			}
			doc, err := html.Parse(bytes.NewReader(data))
			if err != nil {
				return err
			}

			walkOpt := walkHTMLDocumentOptions{
				url: func(urlStr string) {
					if skipURL(urlStr) {
						return
					}

					u, err := url.Parse(urlStr)
					if err != nil {
						problems = append(problems, fmt.Sprintf("%s: invalid URL %q", path, urlStr))
					}

					// Resolve relative URLs.
					if !u.IsAbs() {
						//u = &url.URL{Path}
					}

					rr := httptest.NewRecorder()
					req, _ := http.NewRequest("HEAD", urlStr, nil)
					handler.ServeHTTP(rr, req)
					if rr.Code != http.StatusOK {
						problems = append(problems, fmt.Sprintf("%s: broken link to %s", path, urlStr))
					}
				},
			}

			walkHTMLDocument(doc, walkOpt)
			return nil
		}
		if err := walkFileSystem(gen.Sources, checkFile); err != nil {
			return err
		}

		if len(problems) > 0 {
			for _, problem := range problems {
				fmt.Println(problem)
			}
			return fmt.Errorf("%d problems found", len(problems))
		}
		return nil
	}

	// Register the command.
	commands = append(commands, &command{
		FlagSet:          flagSet,
		ShortDescription: "check all source files for problems",
		LongDescription:  "The check subcommand checks all source files for problems, such as template execution errors and broken links.",
		handler:          handler,
	})
}

type walkHTMLDocumentOptions struct {
	url func(url string) // called for each URL encountered
}

func walkHTMLDocument(node *html.Node, opt walkHTMLDocumentOptions) {
	if node.Type == html.ElementNode {
		switch node.DataAtom {
		case atom.A:
			if href, ok := getAttribute(node, "href"); ok {
				opt.url(href)
			}
		case atom.Img:
			if src, ok := getAttribute(node, "src"); ok {
				opt.url(src)
			}
		}
	}

	for c := node.FirstChild; c != nil; c = c.NextSibling {
		walkHTMLDocument(c, opt)
	}
}

func getAttribute(n *html.Node, key string) (string, bool) {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val, true
		}
	}
	return "", false
}
