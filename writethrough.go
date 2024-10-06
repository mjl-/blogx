package main

import (
	"os"
	"path"
)

func removeWritethrough(filename string) {
	if filename != "" {
		os.Remove(filename)
		os.Remove(path.Dir(filename))
	}
	os.Remove("data/www/index.html")
	os.Remove("data/www/feed.atom")
}
