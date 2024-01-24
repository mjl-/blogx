package main

import (
	"fmt"
	mathrand "math/rand"
	"os"
	"path/filepath"
	"time"
)

var idgen = mathrand.New(mathrand.NewSource(time.Now().UnixNano()))

func newID() string {
	const characters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"


	buf := make([]byte, 16)
	for i := range buf {
		buf[i] = characters[idgen.Intn(len(characters))]
	}
	return string(buf)
}

type writer struct {
	f *os.File
}

type writeError struct{ error }

func (w *writer) check(err error, action string) {
	if err != nil {
		w.errorf("%s: %s", action, err)
	}
}

func (w *writer) errorf(format string, args ...interface{}) {
	panic(writeError{fmt.Errorf(format, args...)})
}

func (w *writer) handle(err *error) {
	e := recover()
	if e == nil {
		return
	}
	ee, ok := e.(writeError)
	if ok {
		*err = ee
	} else {
		panic(e)
	}
}

func (w *writer) Linef(format string, args ...interface{}) {
	w.Text(fmt.Sprintf(format, args...) + "\n")
}

func (w *writer) Time(tm time.Time) {
	w.Text(tm.Format(time.RFC3339) + "\n")
}

func (w *writer) Text(s string) {
	_, err := w.f.Write([]byte(s))
	w.check(err, "write")
}

func writePost(p *post) (rerr error) {
	w := &writer{}
	defer w.handle(&rerr)

	if p.ID == "" {
		w.errorf("missing ID")
	}
	path := fmt.Sprintf("data/post/%s/post.txt", p.ID)
	os.MkdirAll(filepath.Dir(path), 0777)
	f, err := os.Create(path)
	w.check(err, "create post file")
	w.f = f
	w.Linef("v1")
	w.Linef(p.ID)
	if p.Active {
		w.Linef("active")
	} else {
		w.Linef("inactive")
	}
	w.Linef(p.Slug)
	w.Linef(p.Title)
	w.Time(p.Time)
	w.Linef("body:")
	w.Text(p.Body)
	err = f.Close()
	w.check(err, "close post")
	return
}

func writeComment(c *comment) (rerr error) {
	w := &writer{}
	defer w.handle(&rerr)

	if c.ID == "" {
		w.errorf("missing ID")
	}
	if c.PostID == "" {
		w.errorf("empty PostID on comment")
	}
	path := fmt.Sprintf("data/post/%s/comment/%s.txt", c.PostID, c.ID)
	os.MkdirAll(filepath.Dir(path), 0777)
	f, err := os.Create(path)
	w.check(err, "create comment file")
	w.f = f
	w.Linef("v1")
	w.Linef(c.ID)
	if c.Active {
		w.Linef("active")
	} else {
		w.Linef("inactive")
	}
	if c.Seen {
		w.Linef("seen")
	} else {
		w.Linef("notseen")
	}
	w.Time(c.Time)
	w.Linef(c.Author)
	w.Linef("body:")
	w.Text(c.Body)
	err = f.Close()
	w.check(err, "close comment")
	return
}

func writeImage(img *image) (rerr error) {
	w := &writer{}
	defer w.handle(&rerr)

	if img.ID == "" {
		w.errorf("missing ID")
	}
	path := fmt.Sprintf("data/image/%s/image.txt", img.ID)
	os.MkdirAll(filepath.Dir(path), 0777)
	f, err := os.Create(path)
	w.check(err, "create image file")
	w.f = f
	w.Linef("v1")
	w.Linef(img.ID)
	w.Linef(img.Slug)
	w.Linef(img.Title)
	w.Time(img.Time)
	w.Linef(img.Mimetype)
	w.Linef(img.Filename)
	err = f.Close()
	w.check(err, "close image")
	return
}

func writeImageData(img *image, data []byte) (rerr error) {
	w := &writer{}
	defer w.handle(&rerr)

	path := fmt.Sprintf("data/image/%s/%s", img.ID, img.Filename)
	os.MkdirAll(filepath.Dir(path), 0777)

	df, err := os.Create(path)
	w.check(err, "create image data file")
	_, err = df.Write(data)
	w.check(err, "writing image data")
	err = df.Close()
	w.check(err, "closing image data")
	return
}
