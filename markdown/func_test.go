package markdown

import (
	"reflect"
	"strconv"
	"testing"

	"golang.org/x/net/html"
)

func TestGetMarkdownFuncInvocation(t *testing.T) {
	tests := []struct {
		tok          html.Token
		wantFuncName string
		wantArgs     map[string]string
	}{
		{
			tok: html.Token{
				Attr: []html.Attribute{},
			},
			wantFuncName: "",
			wantArgs:     nil,
		},
		{
			tok: html.Token{
				Attr: []html.Attribute{{Key: "x", Val: "y"}},
			},
			wantFuncName: "",
			wantArgs:     nil,
		},
		{
			tok: html.Token{
				Attr: []html.Attribute{{Key: "markdown-func", Val: "f"}},
			},
			wantFuncName: "f",
			wantArgs:     nil,
		},
		{
			tok: html.Token{
				Attr: []html.Attribute{{Key: "f:a", Val: "1"}, {Key: "markdown-func", Val: "f"}, {Key: "f:b", Val: "2"}},
			},
			wantFuncName: "f",
			wantArgs:     map[string]string{"a": "1", "b": "2"},
		},
	}
	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			funcName, args := getMarkdownFuncInvocation(test.tok)
			if funcName != test.wantFuncName {
				t.Errorf("got funcName %q, want %q", funcName, test.wantFuncName)
			}
			if !reflect.DeepEqual(args, test.wantArgs) {
				t.Errorf("got args %q, want %q", args, test.wantArgs)
			}
		})
	}
}
