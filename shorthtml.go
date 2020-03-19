package main

import (
	"bytes"
	"golang.org/x/net/html"
	"io"
	"strings"
)

// after parsing, html/head/body etc tags have been added.
// we only want to print what's inside the body element.
func renderBody(r *html.Node, w io.Writer) error {
	switch r.Type {
	case html.DocumentNode, html.ElementNode:
		switch r.Data {
		case "body":
			for c := r.FirstChild; c != nil; c = c.NextSibling {
				err := html.Render(w, c)
				if err != nil {
					return err
				}
			}
		default:
			for c := r.FirstChild; c != nil; c = c.NextSibling {
				err := renderBody(c, w)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

type trunc struct {
	left int
}

func (t *trunc) truncate(n *html.Node) {
	switch n.Type {
	case html.ErrorNode:
	case html.CommentNode:
	case html.DoctypeNode:

	case html.TextNode:
		nn := len(n.Data)
		if t.left < nn {
			nn = t.left
			n.Data = n.Data[:nn] + "..."
		}
		t.left -= nn
	case html.DocumentNode, html.ElementNode:
		c := n.FirstChild
		for c != nil {
			t.truncate(c)
			if t.left == 0 {
				c.NextSibling = nil
			}
			c = c.NextSibling
		}
	}
}

// parse s as html, and output it again as html, but stop when reaching certain length.
// html tags are properly closed.  text is cut off and "..." appended.
// the new html is returned, or an error.
func htmltrunc(s string) (string, error) {
	r, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return "", err
	}
	t := &trunc{275}
	t.truncate(r)
	w := new(bytes.Buffer)
	err = renderBody(r, w)
	if err != nil {
		return "", err
	}
	return w.String(), nil
}
