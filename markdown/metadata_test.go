package markdown

import "testing"

func TestMetadata(t *testing.T) {
	t.Run("parseMetadata", func(t *testing.T) {
		t.Run("OK", func(t *testing.T) {
			frontMatter := `---
category: article
---
`
			meta, _, err := parseMetadata([]byte(frontMatter))
			if err != nil {
				t.Fatal(err)
			}
			if want := "article"; want != meta.Category {
				t.Errorf("got data %q, want %q", meta.Category, want)
			}
		})

		t.Run("NOK unknown keys", func(t *testing.T) {
			frontMatter := `---
Category: article
---
`
			_, _, err := parseMetadata([]byte(frontMatter))
			if err == nil {
				t.Errorf("got no error, want not nil err")
			}
		})
	})
}
