package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"time"
)

var errNoID = errors.New("no id")

type store struct {
	Posts  []*post
	Images []*image
}

type post struct {
	ID     string
	Active bool
	Slug   string
	Title  string
	Time   time.Time
	Body   string

	Comments []*comment
}

type comment struct {
	ID     string
	PostID string
	Active bool
	Seen   bool
	Time   time.Time
	Author string
	Body   string
}

type image struct {
	ID       string
	Time     time.Time
	Slug     string
	Title    string
	Filename string
	Mimetype string // eg image/jpeg
}

func (img *image) Data() ([]byte, error) {
	return ioutil.ReadFile("data/image/" + img.ID + "/" + img.Filename)
}

func readStore() (st *store, rerr error) {
	defer func() {
		e := recover()
		if e != nil {
			ee, ok := e.(parseError)
			if ok {
				rerr = ee
			} else {
				panic(e)
			}
		}
	}()

	var posts []*post
	var images []*image

	l, err := ioutil.ReadDir("data/post")
	if err != nil {
		return nil, fmt.Errorf("listing posts: %s", err)
	}
	for _, fi := range l {
		dir := "data/post/" + fi.Name()
		p := readPost(dir+"/post.txt", fi.Name())
		sort.Slice(p.Comments, func(i, j int) bool {
			return p.Comments[i].Time.After(p.Comments[j].Time)
		})
		posts = append(posts, p)
	}
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Time.After(posts[j].Time)
	})

	l, err = ioutil.ReadDir("data/image")
	if err != nil {
		return nil, fmt.Errorf("listing images: %s", err)
	}
	for _, fi := range l {
		dir := "data/image/" + fi.Name()
		img := readImage(dir+"/image.txt", fi.Name())
		images = append(images, img)
	}
	sort.Slice(images, func(i, j int) bool {
		return images[i].Time.After(images[j].Time)
	})

	return &store{posts, images}, nil
}

func (s *store) post(id string) *post {
	for _, p := range s.Posts {
		if p.ID == id {
			return p
		}
	}
	abort(404)
	return nil // not reached
}

func (s *store) findPostBySlug(slug string) *post {
	for _, p := range s.Posts {
		if p.Slug == slug {
			return p
		}
	}
	return nil
}

func (s *store) comment(commentID string) (*post, *comment) {
	for _, p := range s.Posts {
		for _, c := range p.Comments {
			if c.ID == commentID {
				return p, c
			}
		}
	}
	abort(404)
	return nil, nil // not reached
}

func (s *store) findImageBySlug(slug string) *image {
	for _, img := range s.Images {
		if img.Slug == slug {
			return img
		}
	}
	return nil
}

func deletePost(p *post) error {
	if p.ID == "" {
		return errNoID
	}
	return os.RemoveAll(fmt.Sprintf("data/post/%s", p.ID))
}

func deleteComment(c *comment) error {
	return os.Remove(fmt.Sprintf("data/post/%s/comment/%s.txt", c.PostID, c.ID))
}

type parseError struct{ error }

type parser struct {
	r *bufio.Reader
}

func (p *parser) check(err error, action string) {
	if err != nil {
		p.errorf("%s: %s", action, err)
	}
}

func (p *parser) errorf(format string, args ...interface{}) {
	panic(parseError{fmt.Errorf(format, args...)})
}

func (p *parser) readline() string {
	line, err := p.r.ReadString('\n')
	p.check(err, "reading line")
	if !strings.HasSuffix(line, "\n") {
		p.check(io.ErrUnexpectedEOF, "reading line")
	}
	return line[:len(line)-1]
}

func (p *parser) Literal(s string) {
	text := p.readline()
	if s != text {
		p.errorf("got %q, expected literal %q", text, s)
	}
}

func (p *parser) ID(id string) {
	text := p.readline()
	if id != text {
		p.errorf("got %q, expected id %q", text, id)
	}
}

func (p *parser) Bool(v *bool, falseVal, trueVal string) {
	text := p.readline()
	switch text {
	case falseVal:
		*v = false
	case trueVal:
		*v = true
	default:
		p.errorf("got %q, expected boolean %q or %q", text, falseVal, trueVal)
	}
}

func (p *parser) Line(v *string) {
	*v = p.readline()
}

func (p *parser) Time(v *time.Time) {
	s := p.readline()
	t, err := time.Parse(time.RFC3339, s)
	p.check(err, "parsing time")
	*v = t
}

func (p *parser) Text(line string, v *string) {
	s := p.readline()
	if s != line {
		p.errorf("got %q, expected start of text marker %q", s, line)
	}
	buf, err := ioutil.ReadAll(p.r)
	p.check(err, "reading remaining text")
	*v = string(buf)
}

func (p *parser) EOF() {
	buf, err := ioutil.ReadAll(p.r)
	p.check(err, "reading for eof")
	if len(buf) != 0 {
		p.errorf("got %q, expected eof", string(buf))
	}
}

func readPost(filename string, id string) (po *post) {
	po = &post{ID: id}

	p := &parser{}
	f, err := os.Open(filename)
	p.check(err, "open post file")
	defer f.Close()

	p.r = bufio.NewReader(f)

	p.Literal("v1")
	p.ID(po.ID)
	p.Bool(&po.Active, "inactive", "active")
	p.Line(&po.Slug)
	p.Line(&po.Title)
	p.Time(&po.Time)
	p.Text("body:", &po.Body)

	commentDir := fmt.Sprintf("data/post/%s/comment", po.ID)
	l, err := ioutil.ReadDir(commentDir)
	if err != nil && os.IsNotExist(err) {
		return
	}
	p.check(err, "listing comments")
	for _, fi := range l {
		commentID := strings.TrimSuffix(fi.Name(), ".txt")
		c := readComment(commentDir+"/"+fi.Name(), po.ID, commentID)
		po.Comments = append(po.Comments, c)
	}

	return
}

func readComment(filename string, postID, id string) (c *comment) {
	c = &comment{ID: id, PostID: postID}

	p := &parser{}
	f, err := os.Open(filename)
	p.check(err, "open comment file")
	defer f.Close()

	p.r = bufio.NewReader(f)

	p.Literal("v1")
	p.ID(c.ID)
	p.Bool(&c.Active, "inactive", "active")
	p.Bool(&c.Seen, "notseen", "seen")
	p.Time(&c.Time)
	p.Line(&c.Author)
	p.Text("body:", &c.Body)

	return
}

func readImage(filename string, id string) (img *image) {
	img = &image{ID: id}

	p := &parser{}
	f, err := os.Open(filename)
	p.check(err, "open image file")
	defer f.Close()

	p.r = bufio.NewReader(f)

	p.Literal("v1")
	p.ID(img.ID)
	p.Line(&img.Slug)
	p.Line(&img.Title)
	p.Time(&img.Time)
	p.Line(&img.Mimetype)
	p.Line(&img.Filename)
	p.EOF()

	_, err = os.Stat(fmt.Sprintf("data/image/%s/%s", img.ID, img.Filename))
	p.check(err, "checking existence of image data file")

	return
}
