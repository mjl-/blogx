// Package blogx is a simple blog system with completely standalone pages
//
// It has a special feature: all pages include all the content of a page.
// To render a page, the browser does not have to fetch images, stylesheets or javascript to render the page.
//
// Blogx was created as a Go learning exercise.
package main

import (
	"flag"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	textTemplate "text/template"

	"github.com/mjl-/httpasset"
	"github.com/mjl-/sconf"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	version = "dev"

	funcs     template.FuncMap
	textFuncs textTemplate.FuncMap

	baseURL *url.URL
	httpFS  = httpasset.Init("assets")
)

var config struct {
	Password      string
	BaseURL       string
	CookieAuthKey string
	BlogTitle     string
	SecureCookies bool
}

func main() {
	if len(os.Args) < 2 {
		log.Println("usage: blogx { config-test | config-describe | serve | version")
		os.Exit(2)
	}

	cmd := os.Args[1]
	args := os.Args[2:]
	switch cmd {
	case "config-test":
		if len(args) != 1 {
			log.Fatalf("usage: blogx config-test blogx.conf")
		}
		err := sconf.ParseFile(args[0], &config)
		check(err, "parsing config file")
	case "config-describe":
		if len(args) != 0 {
			log.Fatalf("usage: blogx config-describe")
		}
		err := sconf.Describe(os.Stdout, &config)
		check(err, "describing config file")
	case "serve":
		serve(args)
	case "version":
		log.Printf("version %s", version)
	default:
		flag.Usage()
		os.Exit(2)
	}
}

func serve(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "localhost:5011", "Address to listen on")
	listenAdmin := fs.String("listenadmin", "localhost:5012", "Address to listen on for admin handlers like prometheus metrics")
	fs.Usage = func() {
		log.Printf("usage: blogx serve blogx.conf")
		fs.PrintDefaults()
	}
	fs.Parse(args)
	args = fs.Args()
	if len(args) != 1 {
		fs.Usage()
		os.Exit(2)
	}

	err := sconf.ParseFile(args[0], &config)
	check(err, "parsing config file")

	baseURL, err = url.Parse(config.BaseURL)
	check(err, "parsing baseURL in config file")

	stripBase := func(fn http.Handler) http.Handler {
		return http.StripPrefix(baseURL.Path, fn)
	}

	http.Handle("/metrics", promhttp.Handler())

	mux := http.NewServeMux()
	mux.Handle(baseURL.Path+"s/", stripBase(http.FileServer(httpFS)))
	mux.Handle(baseURL.Path+"p/", handleHTTPError(stripBase(http.HandlerFunc(publicPost))))
	mux.Handle(baseURL.Path+"a/", handleHTTPError(stripBase(http.HandlerFunc(admin))))
	mux.Handle(baseURL.Path, handleHTTPError(stripBase(http.HandlerFunc(index))))

	log.Printf("blogx %s listening on %s and %s, see %s", version, *addr, *listenAdmin, config.BaseURL)
	go func() {
		log.Fatal(http.ListenAndServe(*listenAdmin, nil))
	}()
	log.Fatal(http.ListenAndServe(*addr, mux))
}
