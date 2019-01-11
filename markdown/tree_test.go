package markdown

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestNewTree(t *testing.T) {
	ast := NewParser(NewBfRenderer()).Parse([]byte(`# 1a
## 2a
### 3a
#### 4
##### 5a
###### 6a
###### 6b
##### 5b
### 3b
##### 5c

# 1b
## 2b`))
	tree := newTree(ast)
	want := []*SectionNode{
		{
			Title: "1a", URL: "#1a", Level: 1,
			Children: []*SectionNode{
				{
					Title: "2a", URL: "#2a", Level: 2,
					Children: []*SectionNode{
						{
							Title: "3a", URL: "#3a", Level: 3,
							Children: []*SectionNode{
								{
									Title: "4", URL: "#4", Level: 4,
									Children: []*SectionNode{
										{
											Title: "5a", URL: "#5a", Level: 5,
											Children: []*SectionNode{
												{Title: "6a", URL: "#6a", Level: 6},
												{Title: "6b", URL: "#6b", Level: 6},
											},
										},
										{Title: "5b", URL: "#5b", Level: 5},
									},
								},
							},
						},
						{
							Title: "3b", URL: "#3b", Level: 3,
							Children: []*SectionNode{
								{Title: "5c", URL: "#5c", Level: 5},
							},
						},
					},
				},
			},
		},
		{
			Title: "1b", URL: "#1b", Level: 1,
			Children: []*SectionNode{
				{Title: "2b", URL: "#2b", Level: 2},
			},
		},
	}
	if !reflect.DeepEqual(tree, want) {
		a, _ := json.MarshalIndent(tree, "", "  ")
		b, _ := json.MarshalIndent(want, "", "  ")
		t.Errorf("\ngot:\n%s\n\nwant:\n%s", a, b)
	}
}

func TestNewTree_link(t *testing.T) {
	ast := NewParser(NewBfRenderer()).Parse([]byte(`# [A](B)`))
	tree := newTree(ast)
	want := []*SectionNode{
		{
			Title: "A", URL: "B", Level: 1,
		},
	}
	if !reflect.DeepEqual(tree, want) {
		a, _ := json.MarshalIndent(tree, "", "  ")
		b, _ := json.MarshalIndent(want, "", "  ")
		t.Errorf("\ngot:\n%s\n\nwant:\n%s", a, b)
	}
}
