package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func generate(w http.ResponseWriter, args map[string]interface{}, templatePaths ...string) {
	t, err := template.New("x").Parse("x")
	httpCheck(err)
	t = t.Funcs(funcs)
	httpCheck(err)
	paths := append([]string{"t/admin.html"}, templatePaths...)
	for _, path := range paths {
		f, err := httpFS.Open("/" + path)
		httpCheck(err)
		defer f.Close()
		templ, err := ioutil.ReadAll(f)
		httpCheck(err)
		t, err = t.Parse(string(templ))
		httpCheck(err)
	}

	httpCheck(err)
	httpCheck(t.ExecuteTemplate(w, "admin", args))
}

func admin(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "a/" && r.Method == "GET" {
		http.Redirect(w, r, fmt.Sprintf("%sa/index/", config.BaseURL), 302)
		return
	}

	elems := strings.Split(r.URL.Path, "/")
	elems = elems[1:] // skip the "a"
	if len(elems) != 2 {
		abort(404)
	}

	params := strings.Split(elems[1], ",")
	if elems[1] == "" {
		params = []string{}
	}

	paramsNeed := func(n int) {
		if len(params) != n {
			abortUserError("Wrong number of parameters.")
		}
	}

	parseTime := func(s string) time.Time {
		tm, err := time.Parse(time.RFC3339, s)
		httpCheck(err)
		return tm
	}

	setAuthCookie := func() {
		value := generateAuth([]byte(config.CookieAuthKey + config.Password))
		cookie := &http.Cookie{Name: "auth", Value: value, Path: baseURL.Path, MaxAge: 12 * 3600, Secure: config.SecureCookies, HttpOnly: true}
		http.SetCookie(w, cookie)
	}

	removeAuthCookie := func() {
		cookie := &http.Cookie{Name: "auth", Path: baseURL.Path, MaxAge: -1, Secure: config.SecureCookies, HttpOnly: true}
		http.SetCookie(w, cookie)
	}

	data, err := readStore()
	httpCheck(err)

	cmd := elems[0]
	args := map[string]interface{}{}
	if cmd != "login" {
		cookie, err := r.Cookie("auth")
		if err != nil {
			next := fmt.Sprintf("%s%s", config.BaseURL, r.URL.Path)
			http.Redirect(w, r, fmt.Sprintf("%sa/login/?next=%s", config.BaseURL, url.QueryEscape(next)), 303)
			return
		}
		verifyAuth([]byte(config.CookieAuthKey+config.Password), cookie.Value)

		// Verify right intentions of request.
		if r.Method == "POST" {
			csrf := r.FormValue("csrf")
			if csrf == "" {
				abortUserError("Bad CSRF.")
			}
			verifyAuth([]byte(config.CookieAuthKey), csrf)
		}
		setAuthCookie()
	}

	switch cmd {
	case "index":
		needGet(r)
		paramsNeed(0)
		args["posts"] = data.Posts
		generate(w, args, "t/admin/index.html")

	case "post":
		needGet(r)
		paramsNeed(1)
		args["post"] = data.post(params[0])
		generate(w, args, "t/admin/post.html")

	case "post-create":
		needPost(r)
		paramsNeed(0)
		p := &post{
			ID:     newID(),
			Active: false,
			Time:   time.Now(),
			Body:   "",
		}
		p.Slug = r.PostFormValue("slug")
		p.Title = r.PostFormValue("title")
		if data.findPostBySlug(p.Slug) != nil {
			abortUserError("Slug already exists.")
		}
		err = writePost(p)
		httpCheck(err)
		removeWritethrough("")
		http.Redirect(w, r, fmt.Sprintf("%sa/post/%s", config.BaseURL, p.ID), 303)

	case "post-save":
		needPost(r)
		paramsNeed(1)
		p := data.post(params[0])
		p.Active = r.PostFormValue("active") != ""
		slug := r.PostFormValue("slug")
		if slug == "" {
			abortUserError("Empty slug is invalid.")
		}
		oldSlug := p.Slug
		if slug != p.Slug && data.findPostBySlug(slug) != nil {
			abortUserError("New slug already exists.")
		}
		p.Slug = slug
		p.Title = r.PostFormValue("title")
		p.Time = parseTime(r.PostFormValue("time"))
		p.Body = r.PostFormValue("body")
		err = writePost(p)
		httpCheck(err)
		if p.Slug != oldSlug {
			removeWritethrough(fmt.Sprintf("data/www/p/%s/index.html", oldSlug))
		}
		removeWritethrough(fmt.Sprintf("data/www/p/%s/index.html", p.Slug))
		http.Redirect(w, r, fmt.Sprintf("%sa/post/%s", config.BaseURL, p.ID), 303)

	case "post-delete":
		needPost(r)
		paramsNeed(1)
		p := data.post(params[0])
		err = deletePost(p)
		httpCheck(err)
		removeWritethrough(fmt.Sprintf("data/www/p/%s/index.html", p.Slug))
		http.Redirect(w, r, fmt.Sprintf("%sa/index/", config.BaseURL), 303)

	case "post-preview":
		needPost(r)
		paramsNeed(1)
		p := data.post(params[0])

		cthtml(w)
		cc, ww := compact(w)
		httpCheck(parseTemplate("t/post.html").Execute(ww, map[string]interface{}{
			"post": p,
		}))
		ww.Close()
		<-cc

	case "comment-seen":
		needPost(r)
		paramsNeed(1)
		p, c := data.comment(params[0])
		c.Seen = true
		err = writeComment(c)
		httpCheck(err)
		removeWritethrough(fmt.Sprintf("data/www/p/%s/index.html", p.Slug))
		http.Redirect(w, r, fmt.Sprintf("%sa/post/%s", config.BaseURL, c.PostID), 303)

	case "comment-active":
		needPost(r)
		paramsNeed(1)
		p, c := data.comment(params[0])
		c.Active = r.PostFormValue("active") == "yes"
		err = writeComment(c)
		httpCheck(err)
		removeWritethrough(fmt.Sprintf("data/www/p/%s/index.html", p.Slug))
		http.Redirect(w, r, fmt.Sprintf("%sa/post/%s", config.BaseURL, c.PostID), 303)

	case "comment-delete":
		needPost(r)
		paramsNeed(1)
		p, c := data.comment(params[0])
		err = deleteComment(c)
		httpCheck(err)
		removeWritethrough(fmt.Sprintf("data/www/p/%s/index.html", p.Slug))
		http.Redirect(w, r, fmt.Sprintf("%sa/post/%s", config.BaseURL, c.PostID), 303)

	case "images":
		needGet(r)
		paramsNeed(0)
		args["images"] = data.Images
		generate(w, args, "t/admin/images.html")

	case "image-create":
		needPost(r)
		paramsNeed(0)
		f, fh, err := r.FormFile("image")
		httpCheck(err)
		defer f.Close()

		mimetype := ""
		var ext string
		switch {
		case strings.HasSuffix(fh.Filename, ".jpg"):
			mimetype = "image/jpeg"
			ext = "jpg"
		case strings.HasSuffix(fh.Filename, ".png"):
			mimetype = "image/png"
			ext = "png"
		case strings.HasSuffix(fh.Filename, ".gif"):
			mimetype = "image/gif"
			ext = "gif"
		case strings.HasSuffix(fh.Filename, ".mp4"):
			mimetype = "video/mp4"
			ext = "mp4"
		default:
			abortUserError("Unknown image file extension, please upload a .jpg, .png, .gif or .mp4.")
		}
		buf, err := ioutil.ReadAll(f)
		httpCheck(err)
		img := &image{
			ID:       newID(),
			Time:     time.Now(),
			Slug:     r.FormValue("slug"),
			Title:    r.FormValue("title"),
			Mimetype: mimetype,
			Filename: "data." + ext,
		}
		err = writeImage(img)
		httpCheck(err)
		err = writeImageData(img, buf)
		httpCheck(err)

		http.Redirect(w, r, fmt.Sprintf("%sa/images/", config.BaseURL), 303)

	case "login":
		switch r.Method {
		case "GET":
			args["next"] = r.FormValue("next")
			args["login"] = true
			generate(w, args, "t/admin/login.html")

		case "POST":
			password := r.PostFormValue("password")
			if password != config.Password {
				http.Redirect(w, r, fmt.Sprintf("%sa/login/", config.BaseURL), 303)
				return
			}
			setAuthCookie()
			next := r.PostFormValue("next")
			if next == "" {
				next = fmt.Sprintf("%sa/index/", config.BaseURL)
			}
			http.Redirect(w, r, next, 303)

		default:
			abort(405)
		}

	case "logout":
		needPost(r)
		paramsNeed(0)
		removeAuthCookie()
		http.Redirect(w, r, fmt.Sprintf("%sa/login/", config.BaseURL), 303)

	default:
		abort(404)
	}
}
