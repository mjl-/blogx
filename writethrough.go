package main

import (
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
)

func writethrough(filename string, w io.Writer) (io.Writer, *os.File) {
	os.MkdirAll(path.Dir(filename), 0771)
	wtf, err := os.Create(filename)
	if err != nil {
		log.Println("writethrough:", err)
		return io.MultiWriter(w, ioutil.Discard), nil
	}
	return io.MultiWriter(w, wtf), wtf
}

func removeWritethrough(filename string) {
	if filename != "" {
		os.Remove(filename)
		os.Remove(path.Dir(filename))
	}
	os.Remove("data/www/index.html")
}
