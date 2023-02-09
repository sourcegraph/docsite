package markdown

import (
	"fmt"

	"github.com/shurcooL/sanitized_anchor_name"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/util"
)

func setHeadingIDs() parser.IDs {
	return &headingIDs{values: map[string]struct{}{}}
}

type headingIDs struct {
	values map[string]struct{}
}

func (ids *headingIDs) Generate(value []byte, kind ast.NodeKind) []byte {
	if kind != ast.KindHeading {
		return value
	}

	value = util.TrimLeftSpace(value)
	value = util.TrimRightSpace(value)
	if len(value) == 0 {
		value = util.StringToReadOnlyBytes("heading")
	} else {
		value = util.StringToReadOnlyBytes(sanitized_anchor_name.Create(util.BytesToReadOnlyString(value)))
	}

	if _, ok := ids.values[util.BytesToReadOnlyString(value)]; !ok {
		ids.values[util.BytesToReadOnlyString(value)] = struct{}{}
		return value
	}
	for i := 1; ; i++ {
		newValue := fmt.Sprintf("%s-%d", value, i)
		if _, ok := ids.values[newValue]; !ok {
			ids.values[newValue] = struct{}{}
			return []byte(newValue)
		}
	}
}

func (ids *headingIDs) Put(value []byte) {
	ids.values[util.BytesToReadOnlyString(value)] = struct{}{}
}
