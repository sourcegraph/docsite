package docsite

import (
	"github.com/google/go-cmp/cmp"
	"testing"
)

func TestPatchTempalteForSEO(t *testing.T) {
	tt := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "content without robots meta is patched",
			content: `
{{define "seo"}}
  <meta property="og:locale" content="en_EN">

  <!-- Always set a title -->
  {{ if .Content }}
`,
			want: `
{{define "seo"}}
  <meta property="og:locale" content="en_EN">
  <meta name="robots" content="noindex,nofollow">

  <!-- Always set a title -->
  {{ if .Content }}
`,
		},
		{
			name: "content with robots meta is not patched",
			content: `
{{define "seo"}}
  <meta property="og:locale" content="en_EN">
  <meta name="robots" content="noindex,nofollow">

  <!-- Always set a title -->
  {{ if .Content }}
`,
			want: `
{{define "seo"}}
  <meta property="og:locale" content="en_EN">
  <meta name="robots" content="noindex,nofollow">

  <!-- Always set a title -->
  {{ if .Content }}
`,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got := string(patchTemplateForSEO([]byte(tc.content)))

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("want and got mismatch (-want, +got): %s", diff)
			}
		})
	}
}
