package main

import (
	"bytes"
	"fmt"
	"html/template"
	imagelib "image"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

const (
	minute = 60
	hour   = 60 * minute
	day    = 24 * hour
	week   = 7 * day
	year   = 365 * day
)

func mkage(t int64) string {
	switch {
	case t >= 2*year:
		return fmt.Sprintf("%dy", t/year)
	case t >= 2*week:
		return fmt.Sprintf("%dw", t/week)
	case t >= 2*day:
		return fmt.Sprintf("%dd", t/day)
	case t >= 2*hour:
		return fmt.Sprintf("%dh", t/hour)
	case t >= 2*minute:
		return fmt.Sprintf("%dm", t/minute)
	default:
		return "just now"
	}
}

func slug2url(slug string) string {
	return fmt.Sprintf("%sp/%s/", baseURL.Path, slug)
}

func formatDate(tm time.Time) string {
	s := tm.Format("January 2th 2006")
	s = strings.Replace(s, "1th ", "1st ", -1)
	s = strings.Replace(s, "11st ", "11th ", -1)
	s = strings.Replace(s, "2th ", "2nd ", -1)
	s = strings.Replace(s, "12nd ", "12th ", -1)
	s = strings.Replace(s, "3th ", "3rd ", -1)
	s = strings.Replace(s, "13rd ", "13th ", -1)
	return s
}

func inlineCSS(path string) template.CSS {
	f, err := httpFS.Open("/" + path)
	httpCheck(err)
	defer f.Close()
	buf, err := ioutil.ReadAll(f)
	httpCheck(err)
	return template.CSS(string(buf))
}

func image2img(ximage *image) *Img {
	data, err := ximage.Data()
	httpCheck(err)
	img, format, err := imagelib.Decode(bytes.NewBuffer(data))
	httpCheck(err)
	return &Img{img: img, format: format}
}

func imagePath(path string) *Img {
	f, err := os.Open(path)
	httpCheck(err)
	defer f.Close()
	img, format, err := imagelib.Decode(f)
	httpCheck(err)
	return &Img{img: img, format: format}
}

func imageSlug(slug string) *Img {
	return image2img(imageSlugRaw(slug))
}

func imageSlugRaw(slug string) *image {
	data, err := readStore() // todo: don't read this again, it's already in memory somewhere
	httpCheck(err)
	ximage := data.findImageBySlug(slug)
	if ximage == nil {
		abortUserError("Image not found.")
	}
	return ximage
}

func render(templ string) (string, error) {
	b := &bytes.Buffer{}
	err := parseTemplateString("x", templ).Execute(b, map[string]interface{}{})
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

func hasPrefix(prefix, s string) bool {
	return strings.HasPrefix(s, prefix)
}

func activeCommentCount(p *post) (n int) {
	for _, c := range p.Comments {
		if c.Active {
			n++
		}
	}
	return
}
