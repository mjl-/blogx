package main

import (
	"testing"
)

var tab = []struct {
	in  string
	out string
}{
	{"<div>  </div>", "<div></div>"},
	{"<span>  </span>", "<span></span>"},
	{"<div> <span>x</span></div>", "<div><span>x</span></div>"},
	{"<div><span>x</span> </div>", "<div><span>x</span></div>"},
	{"<div><span>x</span> </div>", "<div><span>x</span></div>"},
	{"<div><span> x </span></div>", "<div><span>x </span></div>"},                                   // todo: get rid of the space after x
	{"<div> <span> x </span> </div>", "<div><span>x </span></div>"},                                 // todo: get rid of the space after x
	{"<div><span> x </span></div>", "<div><span>x </span></div>"},                                   // todo: get rid of the space after x
	{"<div><span> x </span> <span> y </span></div>", "<div><span>x </span><span>y </span></div>"},   // todo: get rid of space after y
	{"<div> <span> x </span> <span> y </span></div> ", "<div><span>x </span><span>y </span></div>"}, // todo: get rid of space after y
	{"<div><span>x</span>  <span>y</span></div>", "<div><span>x</span> <span>y</span></div>"},
	{"<div> <span>x</span> </div>", "<div><span>x</span></div>"},
	{"<pre> \n\t  </pre>", "<pre> \n\t  </pre>"},
	{"<pre><div> \n\t  </div></pre>", "<pre><div> \n\t  </div></pre>"},
	{`<div class="test"></div>`, `<div class=test></div>`},
	{`<div class="a=b"></div>`, `<div class="a=b"></div>`},
	{`<div>&lt;i&gt;hi&lt;/i&gt;</div>`, `<div>&lt;i&gt;hi&lt;/i&gt;</div>`},

	// see http://tengine.taobao.org/document/http_trim_filter.html for inspiration
}

func TestCompacthtml(t *testing.T) {
	for i, e := range tab {
		r := Compact(e.in)
		if r != e.out {
			t.Errorf("test %d failed, expected %q, saw %q", i+1, e.out, r)
		}
	}
}
