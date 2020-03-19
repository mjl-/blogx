package main

/*
Compacthtml is a library that parses html, removes needless whitespace,
and writes back cleaner html.  Whitespace removal is done based on
heuristics. It pretends to know which elements are inline elements,
and which elements need to be handled specially to preserve whitespace
(eg pre, textarea, script).  Of course, these properties can be changed
with CSS or in JS. Only use this with HTML where the display styles are
not changed.

Warning: This is a proof of concept.  It's a hack.

A bit about how this works.

First of all, some elements need to keep their whitespace untouched:
pre, textarea, script.

After that, we divide elements in inline and block-level elements. For
inline elements we collapse whitespace, eg multiple space/tab/newline
are collapsed into a single space.  Text with a block-level parent is
trimmed of its whitespace.  Leading and trailing whitespace is sometimes
stripped from text inside an inline element.  For this, we keep track
of the text generated for the current block-level element (an ancestor
in the parse tree).  The first whitespace in a block is dropped. Leading
whitespace is also dropped if the last text outputted was already a space.

A final space in a block could also be dropped, but this is not currently
done, to keep the code simple.

Todo:
- Be more correct. We may be stripping too much whitespace in some cases.
- Find more ways to be more compact.
- Strip comments that don't have special meaning? Would have to figure out which those are first...
*/

import (
	"bytes"
	"fmt"
	"github.com/dchest/cssmin"
	"golang.org/x/net/html"
	"io"
	"regexp"
	"strings"
)

type block struct {
	text       string // text seen so far
	spacedelay bool   // whether space has been delayed, and may have to be printed
}

func (b *block) IsEmpty() bool {
	return b.text == ""
}

func (b *block) EndsWithSpace() bool {
	return strings.HasSuffix(b.text, " ")
}

func (b *block) Add(s string) {
	b.text += s
}

type tag struct {
	name string // element name
	lit  bool   // is literal (based on name and inheritance)
	b    *block // shared in the stack, up to the block-level element
}
type tagstack []tag

func (t *tagstack) Push(s string) {
	l := *t

	var b *block
	if !isInline(s) || len(l) == 0 {
		b = new(block)
	} else {
		b = l[len(l)-1].b
	}

	e := tag{name: s, lit: isLit(s) || (len(l) > 0 && l[len(l)-1].lit), b: b}
	l = append(l, e)
	*t = l
}

func isLit(s string) bool {
	// xxx probably more... and other tricky html stuff
	switch s {
	case "pre":
		return true
	case "textarea", "script", "template":
		return true
	}
	return false
}

func isInline(s string) bool {
	// from https://developer.mozilla.org/en-US/docs/HTML/Inline_elements
	// with script, textarea removed
	switch s {
	case "b", "big", "i", "small", "tt",
		"abbr", "acronym", "cite", "code", "dfn", "em", "kbd", "strong", "samp", "var",
		"a", "bdo", "br", "img", "map", "object", "q", "span", "sub", "sup",
		"button", "input", "label", "select":
		return true
	}
	return false
}

func (t *tagstack) Pop() string {
	l := *t
	l, e := l[:len(l)-1], l[len(l)-1]
	*t = l
	return e.name
}

func (t *tagstack) Peek() string {
	l := *t
	if len(l) == 0 {
		return ""
	}
	return l[len(l)-1].name
}

func (t *tagstack) Block() *block {
	l := *t
	if len(l) == 0 {
		return new(block)
	}
	return l[len(l)-1].b
}

func (t *tagstack) IsLiteral() bool {
	l := *t
	return len(l) != 0 && l[len(l)-1].lit
}

func writeAttr(w io.Writer, a html.Attribute) bool {
	const needescape = " \t\r\n\f\"'=<>`" // from html5 spec
	if a.Val == "" || strings.ContainsAny(a.Val, needescape) {
		fmt.Fprintf(w, ` %s="%s"`, a.Key, html.EscapeString(a.Val))
		return false
	}
	fmt.Fprintf(w, ` %s=%s`, a.Key, a.Val)
	return true
}

func writeAttrs(w io.Writer, l []html.Attribute) bool {
	needspace := false
	for _, a := range l {
		needspace = writeAttr(w, a)
	}
	return needspace
}

type fakeCloser struct {
	io.Reader
}

func (fc *fakeCloser) Close() error {
	return nil
}

// Compact returns a smaller version of html, with whitespace removed.
func Compact(html string) string {
	out := &bytes.Buffer{}
	cc := make(chan int, 1)
	r := &fakeCloser{strings.NewReader(html)}
	compact0(cc, r, out)
	return out.String()
}

// Compacter returns a writer to which the caller must write html.
// Compacter reads that html, minifies it, and writes it to writer w, as passed in by the caller.
// After the last write to w, Compacter sends an abitrary value on the channel.
// Compacters reads and writes in a goroutine and returns immediately.
func Compacter(w io.Writer) (chan int, io.WriteCloser) {
	pr, pw := io.Pipe()
	cc := make(chan int, 1)
	go compact0(cc, pr, w)
	return cc, pw
}

func compact0(cc chan int, r io.ReadCloser, w io.Writer) {
	defer func() {
		defer func() {
			cc <- 1
		}()
		r.Close()
	}()

	z := html.NewTokenizer(r)

	dupws := regexp.MustCompile(`[ \t\r\n]{1,}`)

	stack := make(tagstack, 0)

	isLiteral := func() bool {
		return stack.IsLiteral()
	}
	isStyle := func() bool {
		return stack.Peek() == "style"
	}
	isScript := func() bool {
		return stack.Peek() == "script"
	}

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			err := z.Err()
			if err == io.EOF {
				return
			}
			panic(err)

		case html.TextToken:
			buf := z.Text()
			if isScript() {
				// todo: minify js
				w.Write(buf)
			} else if isStyle() {
				w.Write(cssmin.Minify(buf))
			} else if isLiteral() {
				fmt.Fprint(w, html.EscapeString(string(buf)))
			} else {
				s := string(buf)
				s = dupws.ReplaceAllString(s, " ")
				b := stack.Block()
				if b.IsEmpty() || b.EndsWithSpace() {
					s = strings.TrimLeft(s, " \t\r\n")
				}
				if !isInline(stack.Peek()) && s == " " {
					b.spacedelay = true
					s = ""
				}
				fmt.Fprint(w, html.EscapeString(s))
				b.Add(s)
			}

		case html.StartTagToken:
			t := z.Token()
			b := stack.Block()
			if b.spacedelay && isInline(t.Data) {
				io.WriteString(w, " ")
				b.spacedelay = false
			}
			stack.Push(t.Data)
			fmt.Fprintf(w, "<%s", t.Data)
			writeAttrs(w, t.Attr)
			fmt.Fprint(w, ">")

		case html.EndTagToken:
			t := z.Token()
			b := stack.Block()
			if b.spacedelay && isInline(t.Data) {
				io.WriteString(w, " ")
				b.spacedelay = false
			}
			fmt.Fprintf(w, "</%s>", t.Data)
			stack.Pop()

		case html.SelfClosingTagToken:
			t := z.Token()
			stack.Push(t.Data)
			fmt.Fprintf(w, "<%s", t.Data)
			needspace := writeAttrs(w, t.Attr)
			if needspace {
				io.WriteString(w, " ")
			}
			fmt.Fprint(w, "/>")
			stack.Pop()

		case html.CommentToken:
			t := z.Token()
			fmt.Fprintf(w, "<!--%s-->", t.Data)

		case html.DoctypeToken:
			t := z.Token()
			fmt.Fprintf(w, "<!doctype %s>\n", t.Data)

		default:
			panic("cannot happen")
		}

	}
}
