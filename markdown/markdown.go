package markdown

import (
	"bytes"
	"fmt"
	"io"
	"net/url"

	"github.com/russross/blackfriday"
)

type Options struct {
	Base           *url.URL
	StripURLSuffix string
}

func Run(text []byte, opt Options) []byte {
	renderer := &renderer{
		Options: opt,
		HTMLRenderer: blackfriday.NewHTMLRenderer(blackfriday.HTMLRendererParameters{
			Flags: blackfriday.CommonHTMLFlags,
		}),
	}
	return blackfriday.Run(text, blackfriday.WithExtensions(blackfriday.CommonExtensions|blackfriday.AutoHeadingIDs), blackfriday.WithRenderer(renderer))
}

type renderer struct {
	Options
	*blackfriday.HTMLRenderer
}

func (r *renderer) RenderNode(w io.Writer, node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
	switch node.Type {
	case blackfriday.Heading:
		if status := r.HTMLRenderer.RenderNode(w, node, entering); status != blackfriday.GoToNext {
			return status
		}
		if entering {
			// Extract text content of the heading.
			fmt.Fprintf(w, `<a name="%s" class="anchor" href="#%s" rel="nofollow" aria-hidden="true"></a>`, node.HeadingID, node.HeadingID)
		}
		return blackfriday.GoToNext
	case blackfriday.Link, blackfriday.Image:
		// Bypass the (HTMLRendererParams).AbsolutePrefix field entirely and perform our own URL
		// resolving. This fixes the issue reported in
		// https://github.com/russross/blackfriday/pull/231 where relative URLs starting with "."
		// are not treated as relative URLs.
		if entering {
			if r.Options.Base != nil {
				if dest, err := url.Parse(string(node.LinkData.Destination)); err == nil && !dest.IsAbs() {
					dest = r.Options.Base.ResolveReference(dest)
					node.LinkData.Destination = []byte(dest.String())
				}
			}
			if r.Options.StripURLSuffix != "" {
				node.LinkData.Destination = bytes.TrimSuffix(node.LinkData.Destination, []byte(r.Options.StripURLSuffix))
			}
		}
	}
	return r.HTMLRenderer.RenderNode(w, node, entering)
}
