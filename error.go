package main

import (
	"errors"
	"log"
	"net/http"
)

func check(err error, action string) {
	if err != nil {
		log.Fatalf("%s: %s", action, err)
	}
}

type httpError int
type httpUserError struct {
	error
}
type httpServerError struct {
	error
}

func needPost(r *http.Request) {
	if r.Method != "POST" {
		abort(405)
	}
}

func needGet(r *http.Request) {
	if r.Method != "GET" {
		abort(405)
	}
}

func abort(e int) {
	panic(httpError(e))
}

func abortUserError(s string) {
	panic(httpUserError{errors.New(s)})
}

func httpCheck(err error) {
	if err != nil {
		log.Println(err)
		abort(500)
	}
}

func handleHTTPError(fn http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				switch e := err.(type) {
				case httpError:
					http.Error(w, http.StatusText(int(e)), int(e))
				case httpUserError:
					http.Error(w, e.Error(), http.StatusBadRequest)
				case httpServerError:
					http.Error(w, e.Error(), http.StatusInternalServerError)
				default:
					log.Println("handler error", err)
					panic(err)
				}
			}
		}()
		fn.ServeHTTP(w, r)
	}
}
