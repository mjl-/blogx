package main

import (
	"bytes"

	"github.com/russross/blackfriday"
)

type headerHTMLRenderer struct {
	*blackfriday.Html
}

func (r *headerHTMLRenderer) Header(out *bytes.Buffer, text func() bool, level int, id string) {
	r.Html.Header(out, text, level+1, id)
}

// just like in blackfriday's Markdown func
func headerMarkdown(in []byte) []byte {
	// set up the HTML renderer
	htmlFlags := 0
	htmlFlags |= blackfriday.HTML_USE_XHTML
	htmlFlags |= blackfriday.HTML_USE_SMARTYPANTS
	htmlFlags |= blackfriday.HTML_SMARTYPANTS_FRACTIONS
	htmlFlags |= blackfriday.HTML_SMARTYPANTS_LATEX_DASHES
	renderer := &headerHTMLRenderer{blackfriday.HtmlRenderer(htmlFlags, "", "").(*blackfriday.Html)}

	// set up the parser
	extensions := 0
	extensions |= blackfriday.EXTENSION_NO_INTRA_EMPHASIS
	extensions |= blackfriday.EXTENSION_TABLES
	extensions |= blackfriday.EXTENSION_FENCED_CODE
	extensions |= blackfriday.EXTENSION_AUTOLINK
	extensions |= blackfriday.EXTENSION_STRIKETHROUGH
	extensions |= blackfriday.EXTENSION_SPACE_HEADERS
	extensions |= blackfriday.EXTENSION_HEADER_IDS

	return blackfriday.Markdown(in, renderer, extensions)
}
