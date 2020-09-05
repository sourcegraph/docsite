package docsite

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"testing"

	"github.com/sourcegraph/docsite/internal/search"
	"github.com/sourcegraph/docsite/internal/search/query"
	"github.com/sourcegraph/docsite/markdown"
	"golang.org/x/tools/godoc/vfs/httpfs"
	"golang.org/x/tools/godoc/vfs/mapfs"
)

func TestSearch(t *testing.T) {
	testMarkdownFuncs = markdown.FuncMap{
		"test-func": func(context.Context, markdown.FuncInfo, map[string]string) (string, error) {
			return "xyz", nil
		},
	}
	t.Cleanup(func() { testMarkdownFuncs = nil })

	tests := map[string]struct {
		pages            map[string]string
		wantQueryResults map[string][]string
	}{
		"simple": {
			pages: map[string]string{
				"a.md": "a",
				"b.md": "b",
			},
			wantQueryResults: map[string][]string{
				"a": {"a.md#: a"},
				"b": {"b.md#: b"},
			},
		},
		"boost path match": {
			pages: map[string]string{
				"a.md": "b",
				"b.md": "",
			},
			wantQueryResults: map[string][]string{
				"b": {"b.md", "a.md#: b"},
			},
		},
		"evaluate markdown funcs": {
			pages: map[string]string{
				"a.md": "<div markdown-func=test-func />",
				"b.md": "",
			},
			wantQueryResults: map[string][]string{
				"x": {"a.md#: xyz"},
			},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			site := Site{
				Content: versionedFileSystem{
					"": httpfs.New(mapfs.New(test.pages)),
				},
				Base: &url.URL{Path: "/"},
			}
			for query, wantResults := range test.wantQueryResults {
				t.Run(query, func(t *testing.T) {
					result, err := site.Search(ctx, "", query)
					if err != nil {
						t.Fatal(err)
					}
					gotResults := toResultsList(result)
					if !reflect.DeepEqual(gotResults, wantResults) {
						t.Errorf("got results %v, want %v", gotResults, wantResults)
					}
				})
			}
		})
	}
}

func toResultsList(result *search.Result) []string {
	var l []string
	for _, dr := range result.DocumentResults {
		if len(dr.SectionResults) == 0 {
			l = append(l, string(dr.ID))
		}
		for _, sr := range dr.SectionResults {
			for _, excerpt := range sr.Excerpts {
				l = append(l, fmt.Sprintf("%s#%s: %s", dr.ID, sr.ID, excerpt))
			}
		}
	}
	return l
}

func TestHighlight(t *testing.T) {
	tests := []struct {
		query, text string
		want        string
	}{
		{
			query: "a",
			text:  "a b a",
			want:  "<strong>a</strong> b <strong>a</strong>",
		},
		{
			query: "a",
			text:  "b<a>a",
			want:  "b&lt;<strong>a</strong>&gt;<strong>a</strong>",
		},
		{
			query: "aaaAaaa",
			text:  "\x9f\x92\xa1 aaaaaaA",
			want:  "\x9f\x92\xa1 <strong>aaaaaaA</strong>",
		},
	}
	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			query := query.Parse(test.query)
			got := highlight(query, test.text)
			if string(got) != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
