package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	textTemplate "text/template"
	"time"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
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
	f, err := fsys.Open("assets/" + path)
	httpCheck(err)
	defer f.Close()
	templ, err := io.ReadAll(f)
	httpCheck(err)
	return template.Must(template.New(path).Funcs(funcs).Parse(string(templ)))
}

func parseTemplateString(name, t string) *template.Template {
	return template.Must(template.New(name).Funcs(funcs).Parse(t))
}

func parseTextTemplateString(name, t string) *textTemplate.Template {
	return textTemplate.Must(textTemplate.New(name).Funcs(textFuncs).Parse(t))
}

var emailregexp = regexp.MustCompile(`\b[[:print:]]+@[0-9A-Za-z\.-]+\.[0-9A-Za-z\.-]+\b`)

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

	active := !strings.Contains(body, "http://") && !strings.Contains(body, "https://") && !emailregexp.MatchString(body)
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
	msg := fmt.Sprintf(`From: <%s>
To: <%s>
Subject: %s comment, %s

New comment from %s.
%s
%s

%s
`, config.Mail.From, config.Mail.To, activetext, author, author, r.Referer(), adminurl, body)
	// We may have a mix of \r\n and \n line endings. First canonicalize to \n, then to \r\n.
	msg = strings.ReplaceAll(msg, "\r\n", "\n")
	msg = strings.ReplaceAll(msg, "\n", "\r\n")

	if config.Mail.Host != "" {
		if err := sendMail(msg); err != nil {
			log.Printf("sending mail: %v", err)
		}
	}

	if active {
		http.Redirect(w, r, fmt.Sprintf("%sp/%s/#comment-%s", config.BaseURL, slug, c.ID), http.StatusSeeOther)
	} else {
		http.Redirect(w, r, fmt.Sprintf("%sp/%s/comment", config.BaseURL, slug), http.StatusSeeOther)
	}
}

func sendMail(msg string) error {
	addr := net.JoinHostPort(config.Mail.Host, fmt.Sprintf("%d", config.Mail.Port))
	var client *smtp.Client
	var err error
	if config.Mail.TLS {
		client, err = smtp.DialTLS(addr, nil)
	} else {
		client, err = smtp.Dial(addr)
	}
	if err != nil {
		return fmt.Errorf("smtp client: %v", err)
	}
	defer client.Close()

	if config.Mail.STARTTLS {
		if err := client.StartTLS(nil); err != nil {
			return fmt.Errorf("smtp starttls: %v", err)
		}
	}

	if config.Mail.Username != "" {
		auth := sasl.NewPlainClient("", config.Mail.Username, config.Mail.Password)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %v", err)
		}
	}

	if err := client.SendMail(config.Mail.From, []string{config.Mail.To}, strings.NewReader(msg)); err != nil {
		return fmt.Errorf("smtp send: %v", err)
	}

	client.Quit()

	return nil
}

func publicPost(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("p/"):]

	l := strings.Split(path, "/")
	slug := l[0]
	if len(l) == 1 && (path == "" || filepath.Clean(path) == "/") {
		http.Redirect(w, r, config.BaseURL, http.StatusMovedPermanently)
		return
	}
	if len(l) == 1 && !strings.HasSuffix(path, "/") {
		http.Redirect(w, r, fmt.Sprintf("%sp/%s/", config.BaseURL, slug), http.StatusMovedPermanently)
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

	var b bytes.Buffer
	ch, cw := Compacter(&b)
	err = parseTemplate("t/post.html").Execute(cw, map[string]interface{}{
		"post": p,
	})
	cw.Close()
	<-ch
	httpCheck(err)

	cthtml(w)
	buf := b.Bytes()
	w.Write(buf)

	ppath := fmt.Sprintf("data/www/p/%s/index.html", slug)
	os.MkdirAll(filepath.Dir(ppath), 0755)
	if err := os.WriteFile(ppath, buf, 0644); err != nil {
		log.Printf("writefile: %v", err)
	}
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

	f, err := fsys.Open("assets/t/index.html")
	httpCheck(err)
	defer f.Close()
	templ, err := io.ReadAll(f)
	httpCheck(err)

	var b bytes.Buffer
	ch, cw := Compacter(&b)
	err = template.Must(template.New("t/index.html").Funcs(funcs).Parse(string(templ))).Execute(cw, map[string]interface{}{
		"posts":      posts,
		"olderposts": olderPosts,
	})
	cw.Close()
	<-ch
	httpCheck(err)

	cthtml(w)
	buf := b.Bytes()
	w.Write(buf)

	if err := os.WriteFile("data/www/index.html", buf, 0644); err != nil {
		log.Printf("writefile: %v", err)
	}
}
