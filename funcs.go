package docsite

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/pkg/errors"
	"github.com/sourcegraph/docsite/markdown"
	"github.com/sourcegraph/go-jsonschema/jsonschema"
	"github.com/sourcegraph/jsonschemadoc"
)

// createMarkdownFuncs creates the standard set of Markdown functions expected by documentation
// content. Documentation pages can invoke these functions with special tags such as <div
// markdown-func=myfunc1 myfunc:arg1="foo" />. The only function currently defined is jsonschemadoc,
// which generates documentation for a JSON Schema.
//
// TODO: This is not strictly necessary to use with docsite. It could be extracted to a separate
// package and made optional for callers. It's in this package for simplicity for now (because it is
// used on both https://docs.sourcegraph.com and on the in-product /help pages on Sourcegraph).
func createMarkdownFuncs(site *Site) markdown.FuncMap {
	return markdown.FuncMap{
		"jsonschemadoc": func(ctx context.Context, info markdown.FuncInfo, args map[string]string) (string, error) {
			inputPath := args["path"]
			if inputPath == "" {
				return "", errors.New("no path to JSON Schema file is specified (use <div markdown-func=jsonschemadoc jsonschemadoc:path=PATH>)")
			}

			content, err := site.Content.OpenVersion(ctx, info.Version)
			if err != nil {
				return "", err
			}
			data, err := ReadFile(content, inputPath)
			if err != nil {
				return "", err
			}

			var _, schema *jsonschema.Schema
			if err := json.Unmarshal(data, &schema); err != nil {
				return "", err
			}

			title := inputPath

			// Support JSON references to emit documentation for a sub-definition.
			if ref := args["ref"]; ref != "" {
				if !strings.HasPrefix(ref, "#") {
					return "", fmt.Errorf("invalid JSON Schema reference %q (only URI fragments are supported)", ref)
				}
				u, err := url.Parse(ref)
				if err != nil {
					return "", errors.WithMessage(err, "invalid JSON Schema reference")
				}
				// TODO(sqs): support the general case, not just #/definitions/Foo.
				if !strings.HasPrefix(u.Fragment, "/definitions/") || strings.Count(u.Fragment, "/") != 2 {
					return "", fmt.Errorf("unsupported JSON Schema reference %q (only simple #/defintions/Foo references are supported)", u.Fragment)
				}
				defName := strings.TrimPrefix(u.Fragment, "/definitions/")
				if schema.Definitions == nil || (*schema.Definitions)[defName] == nil {
					return "", fmt.Errorf("unable to resolve JSON Schema reference %q", u.Fragment)
				}
				schema = (*schema.Definitions)[defName]
				title += ref
			}

			out, err := jsonschemadoc.Generate(schema)
			if err != nil {
				return "", err
			}

			doc, err := markdown.Run(ctx, []byte("<h2 class=\"json-schema-doc-heading\"><code>"+title+"</code></h2><div class=\"json-schema-doc pre-wrap\">\n```javascript\n"+string(out)+"\n```\n</div>"), markdown.Options{})
			if err != nil {
				return "", err
			}
			return string(doc.HTML), nil
		},
	}
}
