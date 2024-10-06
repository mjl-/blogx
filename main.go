// Package blogx is a simple blog system with completely standalone pages
//
// It has a special feature: all pages include all the content of a page.
// To render a page, the browser does not have to fetch images, stylesheets or javascript to render the page.
//
// Blogx was created as a Go learning exercise.
package main

import (
	"embed"
	"flag"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	textTemplate "text/template"

	"github.com/mjl-/sconf"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	funcs     template.FuncMap
	textFuncs textTemplate.FuncMap

	baseURL *url.URL
)

//go:embed assets
var fsys embed.FS

var config struct {
	Password      string
	BaseURL       string
	CookieAuthKey string
	BlogTitle     string
	BlogAuthor    string
	SecureCookies bool
	Mail          struct {
		Host     string `sconf:"Host of submission/smtp server."`
		Port     int    `sconf:"Port of submission/smtp server, e.g. 465 for submissions, 587 for submission, 25 for smtp."`
		TLS      bool   `sconf:"Dial with TLS, for submissions on port 465."`
		STARTTLS bool   `sconf:"After starting plain text connection, upgrade to TLS with STARTTLS."`
		Username string `sconf:"If set, username for plain text authentication."`
		Password string `sconf:"Password for authentication."`
		From     string `sconf:"SMTP and message From."`
		To       string `sconf:"SMTP and message To."`
	} `sconf:"optional" sconf-doc:"Send email notifications about new potentially spammy comments with this configuration."`
}

func main() {
	log.SetFlags(0)

	if len(os.Args) < 2 {
		log.Println("usage: blogx { config-test | config-describe | serve | version }")
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
	fl := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fl.String("addr", "localhost:5011", "Address to listen on")
	listenAdmin := fl.String("listenadmin", "localhost:5012", "Address to listen on for admin handlers like prometheus metrics")
	fl.Usage = func() {
		log.Printf("usage: blogx serve blogx.conf")
		fl.PrintDefaults()
	}
	fl.Parse(args)
	args = fl.Args()
	if len(args) != 1 {
		fl.Usage()
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

	sfs, err := fs.Sub(fsys, "assets")
	if err != nil {
		log.Fatalf("fsys sub: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle(baseURL.Path+"s/", stripBase(http.FileServer(http.FS(sfs))))
	mux.Handle(baseURL.Path+"p/", handleHTTPError(stripBase(http.HandlerFunc(publicPost))))
	mux.Handle(baseURL.Path+"a/", handleHTTPError(stripBase(http.HandlerFunc(admin))))
	mux.Handle(baseURL.Path+"feed.atom", handleHTTPError(stripBase(http.HandlerFunc(atomFeed))))
	mux.Handle(baseURL.Path, handleHTTPError(stripBase(http.HandlerFunc(index))))

	log.Printf("blogx %s listening on %s and %s, see %s", version, *addr, *listenAdmin, config.BaseURL)
	go func() {
		log.Fatal(http.ListenAndServe(*listenAdmin, nil))
	}()
	log.Fatal(http.ListenAndServe(*addr, mux))
}
