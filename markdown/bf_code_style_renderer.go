package markdown

import (
	"fmt"
	"io"

	"github.com/russross/blackfriday/v2"
)

type CodeStyleRenderer struct {
	blackfriday.Renderer
}

func (c CodeStyleRenderer) RenderNode(w io.Writer, node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
	if node.Type == blackfriday.CodeBlock {
		lang := string(node.Info)
		if lang == "" {
			lang = "plaintext"
		}

		fmt.Fprintf(w, `<pre class="chroma %s">`, lang)
		walkStatus := c.Renderer.RenderNode(w, node, entering)
		fmt.Fprint(w, "</pre>")
		return walkStatus
	}

	return c.Renderer.RenderNode(w, node, entering)
}
