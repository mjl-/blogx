package main

import (
	"encoding/xml"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/tools/blog/atom"
)

func atomFeed(w http.ResponseWriter, r *http.Request) {
	needGet(r)

	data, err := readStore()
	httpCheck(err)

	posts := []*post{}
	for _, p := range data.Posts {
		if p.Active {
			posts = append(posts, p)
		}
	}

	var updated time.Time
	if len(posts) > 0 {
		updated = posts[0].Time
	}
	feed := atom.Feed{
		Title:   config.BlogTitle,
		ID:      config.BaseURL,
		Link:    []atom.Link{{Href: config.BaseURL}},
		Updated: atom.Time(updated),
		Author:  &atom.Person{Name: config.BlogAuthor},
	}
	for _, p := range posts {
		html, err := renderShortMarkdown(p.Body)
		httpCheck(err)

		href := config.BaseURL + "p/" + p.Slug + "/"
		entry := atom.Entry{
			Title:   p.Title,
			Link:    []atom.Link{{Href: href}},
			ID:      href,
			Updated: atom.Time(p.Time),
			Summary: &atom.Text{
				Type: "html",
				Body: string(html),
			},
		}
		feed.Entry = append(feed.Entry, &entry)
	}

	buf, err := xml.Marshal(feed)
	buf = append([]byte("<?xml version=\"1.0\" encoding=\"utf-8\"?>"), buf...)
	httpCheck(err)
	if err := os.WriteFile("data/www/feed.atom", buf, 0644); err != nil {
		log.Printf("writing atom file: %v", err)
	}
	w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
	w.Write(buf)
}
