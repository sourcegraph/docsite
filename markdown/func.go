package markdown

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func evalMarkdownFuncs(ctx context.Context, htmlFragment []byte, opt Options) ([]byte, error) {
	z := html.NewTokenizer(bytes.NewReader(htmlFragment))
	var buf bytes.Buffer
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			if z.Err() == io.EOF {
				break
			}
			return nil, errors.WithMessage(z.Err(), "while evaluating Markdown function tags")
		}
		tok := z.Token()
		invokedFunc := false
		if (tok.Type == html.StartTagToken || tok.Type == html.SelfClosingTagToken) && tok.DataAtom == atom.Div {
			funcName, args := getMarkdownFuncInvocation(tok)
			if funcName != "" {
				f := opt.Funcs[funcName]
				if f == nil {
					return nil, fmt.Errorf("Markdown function %q is not defined", funcName)
				}

				out, err := func() (out string, err error) {
					defer func() {
						if e := recover(); e != nil {
							err = errors.New(fmt.Sprint(e))
						}
					}()
					return f(ctx, opt.FuncInfo, args)
				}()
				if err != nil {
					return nil, errors.WithMessagef(err, "error in Markdown function %q", funcName)
				}
				buf.WriteString(out)
				invokedFunc = true

				// Remove all contents. This makes it so that the contents can be like "noscript",
				// shown only to users viewing the raw Markdown and not the Markdown as
				// rendered/evaluated by this package.
				if tok.Type == html.StartTagToken {
					if err := consumeUntilCloseTag(z, funcName); err != nil {
						return nil, err
					}
				}
				continue
			}
		}

		if !invokedFunc {
			buf.WriteString(tok.String())
		}
	}
	return buf.Bytes(), nil
}

func getMarkdownFuncInvocation(tok html.Token) (funcName string, args map[string]string) {
	for _, attr := range tok.Attr {
		if attr.Key == "markdown-func" {
			funcName = attr.Val
			break
		}
	}
	if funcName != "" {
		argKeyPrefix := funcName + ":"
		for _, attr := range tok.Attr {
			if strings.HasPrefix(attr.Key, argKeyPrefix) {
				if args == nil {
					args = map[string]string{}
				}
				args[strings.TrimPrefix(attr.Key, argKeyPrefix)] = attr.Val
			}
		}
	}
	return funcName, args
}

func consumeUntilCloseTag(z *html.Tokenizer, funcName string) error {
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			if z.Err() == io.EOF {
				return fmt.Errorf("tag for Markdown function %q is never closed", funcName)
			}
			return errors.WithMessagef(z.Err(), "while scanning for Markdown function close tag %q", funcName)
		}
		tok := z.Token()
		if tok.Type == html.EndTagToken {
			return nil
		}
	}
}
