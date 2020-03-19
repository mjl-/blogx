package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
	"path/filepath"
	"strings"
	textTemplate "text/template"
	"time"
)

func compact(w io.Writer) (chan int, io.WriteCloser) {
	return Compacter(w)
}

func cthtml(w http.ResponseWriter) {
	w.Header().Set("content-type", "text/html; charset=utf-8")
}

func renderText(s string) (string, error) {
	b := &bytes.Buffer{}
	err := parseTextTemplateString("renderText", s).Execute(b, map[string]interface{}{})
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

func renderMarkdown(md string) (template.HTML, error) {
	nmd, err := renderText(md)
	if err != nil {
		return template.HTML(""), err
	}
	r := string(headerMarkdown([]byte(nmd)))
	return template.HTML(r), nil
}

func renderShortMarkdown(md string) (template.HTML, error) {
	md, err := renderText(md)
	if err != nil {
		return template.HTML(""), err
	}
	s := string(headerMarkdown([]byte(md)))
	s, err = htmltrunc(s)
	if err != nil {
		return template.HTML(""), err
	}
	return template.HTML(s), nil
}

func init() {
	funcs = template.FuncMap{
		"date": formatDate,
		"timestamp": func(tm time.Time) string {
			return tm.Format(time.RFC3339)
		},
		"slug2url": slug2url,
		"age": func(tm time.Time) string {
			return mkage(time.Now().Unix() - tm.Unix())
		},
		"inlineCSS":           inlineCSS,
		"inlineImage":         inlineImage,
		"image2img":           image2img,
		"imagePath":           imagePath,
		"imageSlug":           imageSlug,
		"imageSlugRaw":        imageSlugRaw,
		"activeCommentCount":  activeCommentCount,
		"hasPrefix":           hasPrefix,
		"thumbnail":           thumbnail,
		"resize":              resize,
		"render":              render,
		"renderMarkdown":      renderMarkdown,
		"renderShortMarkdown": renderShortMarkdown,
		"csrf": func() template.HTML {
			return template.HTML(fmt.Sprintf(`<input type="hidden" name="csrf" value="%s" />`, generateAuth([]byte(config.CookieAuthKey))))
		},
		"basepath": func() string {
			return baseURL.Path
		},
		"baseurl": func() string {
			return config.BaseURL
		},
		"blogtitle": func() string {
			return config.BlogTitle
		},
		"version": func() string {
			return version
		},
	}
	textFuncs = textTemplate.FuncMap{}
	for k, v := range funcs {
		textFuncs[k] = v
	}
}

func parseTemplate(path string) *template.Template {
	f, err := httpFS.Open("/" + path)
	httpCheck(err)
	defer f.Close()
	templ, err := ioutil.ReadAll(f)
	httpCheck(err)
	return template.Must(template.New(path).Funcs(funcs).Parse(string(templ)))
}

func parseTemplateString(name, t string) *template.Template {
	return template.Must(template.New(name).Funcs(funcs).Parse(t))
}

func parseTextTemplateString(name, t string) *textTemplate.Template {
	return textTemplate.Must(textTemplate.New(name).Funcs(textFuncs).Parse(t))
}

func publicComment(slug string, w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		cc, ww := compact(w)
		empty(w, ww, "Comment saved, thanks.<br/> If your comment is not visible, it probably contained a URL and is awaiting moderation.", slug2url(slug))
		ww.Close()
		<-cc
		return
	}

	needPost(r)

	url := r.PostFormValue("url")
	more := r.PostFormValue("more")
	if url != "" || more != "dontchange" {
		abort(400)
	}
	author := r.PostFormValue("author")
	body := r.PostFormValue("body")
	if body == "" {
		abort(400)
	}
	if author == "" {
		author = "anonymous"
	}

	data, err := readStore()
	httpCheck(err)
	defer removeWritethrough(fmt.Sprintf("data/www/p/%s/index.html", slug))

	p := data.findPostBySlug(slug)
	if p == nil || !p.Active {
		abort(404)
	}

	for _, c := range p.Comments {
		if c.Author == author && c.Body == body {
			abort(400)
		}
	}

	hasURL := strings.Contains(body, "http://") || strings.Contains(body, "https://")
	active := !hasURL
	c := &comment{
		ID:     newID(),
		PostID: p.ID,
		Active: active,
		Seen:   false,
		Time:   time.Now(),
		Author: author,
		Body:   body,
	}
	err = writeComment(c)
	httpCheck(err)

	adminurl := fmt.Sprintf("%sa/post/%s", config.BaseURL, p.ID)
	activetext := "new"
	if !active {
		activetext = "new inactive"
	}
	msg := fmt.Sprintf(`Subject: %s comment, %s

New comment from %s.
%s
%s

%s
`, activetext, author, author, r.Referer(), adminurl, body)
	err = smtp.SendMail("localhost:25", smtp.CRAMMD5Auth("x", "x"), "mechiel@ueber.net", []string{"mechiel@ueber.net"}, []byte(msg))
	if err != nil {
		log.Println("sending email after new comment", err)
	}

	if active {
		http.Redirect(w, r, fmt.Sprintf("%sp/%s/#comment-%s", config.BaseURL, slug, c.ID), 303)
	} else {
		http.Redirect(w, r, fmt.Sprintf("%sp/%s/comment", config.BaseURL, slug), 303)
	}
}

func publicPost(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("p/"):]

	l := strings.Split(path, "/")
	slug := l[0]
	if len(l) == 1 && (path == "" || filepath.Clean(path) == "/") {
		http.Redirect(w, r, config.BaseURL, 301)
		return
	}
	if len(l) == 1 && !strings.HasSuffix(path, "/") {
		http.Redirect(w, r, fmt.Sprintf("%sp/%s/", config.BaseURL, slug), 301)
		return
	}
	if len(l) != 2 {
		abort(404)
	}
	if l[1] == "comment" {
		publicComment(slug, w, r)
		return
	}

	needGet(r)

	data, err := readStore()
	httpCheck(err)
	p := data.findPostBySlug(slug)
	if p == nil || !p.Active {
		abort(404)
	}

	cthtml(w)
	mw, wtf := writethrough(fmt.Sprintf("data/www/p/%s/index.html", slug), w)
	if wtf != nil {
		defer wtf.Close()
	}
	cc, ww := compact(mw)
	httpCheck(parseTemplate("t/post.html").Execute(ww, map[string]interface{}{
		"post": p,
	}))
	ww.Close()
	<-cc
}

// Print mostly empty html page, showing msg (which can contain html) and a link back to url.
func empty(w http.ResponseWriter, ww io.Writer, msg, url string) {
	cthtml(w)
	params := map[string]interface{}{
		"message": template.HTML(msg),
		"url":     url,
	}
	httpCheck(parseTemplate("t/empty.html").Execute(ww, params))
}

func index(w http.ResponseWriter, r *http.Request) {
	needGet(r)
	if r.URL.Path != "" {
		abort(404)
	}

	data, err := readStore()
	httpCheck(err)

	posts := []*post{}
	for _, p := range data.Posts {
		if p.Active {
			posts = append(posts, p)
		}
	}
	olderPosts := []*post{}
	if len(posts) > 10 {
		posts, olderPosts = posts[:10], posts[10:]
	}

	f, err := httpFS.Open("/t/index.html")
	httpCheck(err)
	defer f.Close()
	templ, err := ioutil.ReadAll(f)
	httpCheck(err)
	cthtml(w)

	mw, wtf := writethrough("data/www/index.html", w)
	if wtf != nil {
		defer wtf.Close()
	}
	cc, ww := compact(mw)
	httpCheck(template.Must(template.New("t/index.html").Funcs(funcs).Parse(string(templ))).Execute(ww, map[string]interface{}{
		"posts":      posts,
		"olderposts": olderPosts,
	}))
	ww.Close()
	<-cc
}
