package docsite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"text/template"

	"github.com/mozillazg/go-slugify"
	"github.com/pkg/errors"
	"github.com/sourcegraph/go-jsonschema/jsonschema"
	"github.com/sourcegraph/jsonschemadoc"

	"github.com/sourcegraph/docsite/markdown"
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
	m := markdown.FuncMap{
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

			var schema *jsonschema.Schema
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

			// Capture rendered output
			outputTemplate := `
<h2 id="{{.Slug}}" class="json-schema-doc-heading">
	<a class="anchor" href="#{{.Slug}}" rel="nofollow" aria-hidden="true"></a>
	<code>{{.Title}}</code>
</h2>
<div class="json-schema-doc pre-wrap">

{{.Schema}}
</div>`

			tmpl := template.Must(template.New("template").Parse(outputTemplate))
			vars := struct {
				Title  string
				Slug   string
				Schema string
			}{
				title,
				slugify.Slugify(title),
				"```javascript\n" + out + "\n```",
			}
			var output bytes.Buffer
			if err := tmpl.Execute(&output, vars); err != nil {
				return "", err
			}

			doc, err := markdown.Run(output.Bytes(), markdown.Options{})
			if err != nil {
				return "", err
			}
			return string(doc.HTML), nil
		},
	}
	for name, f := range testMarkdownFuncs {
		m[name] = f
	}
	return m
}

// testMarkdownFuncs can be set by tests to inject Markdown functions.
var testMarkdownFuncs markdown.FuncMap
