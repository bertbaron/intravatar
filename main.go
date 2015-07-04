package main

import (
	"flag"
	"github.com/vharitonsky/iniflags"
	"fmt"
	"net/http"
	"regexp"
)

var (
    dataDir = flag.String("data", "data", "Path to data files relative to current working dir. Default is 'data'.")
    port = flag.Int("port", 8080, "Webserver port number. Default = 8080")
)

func avatarHandler(w http.ResponseWriter, r *http.Request, title string) {
	fmt.Println("Handling avatar request", title, *r)
	fmt.Fprintf(w, "<p>Avatar</p>")
}

var validPath = regexp.MustCompile("^/(avatar)/([a-zA-Z0-9]+)$")

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

func main() {
	iniflags.Parse()
	fmt.Printf("data dir = %s\n", *dataDir)
	address := fmt.Sprintf(":%d", *port)
	
	http.HandleFunc("/avatar/", makeHandler(avatarHandler))
	x := http.ListenAndServe(address, nil)
	fmt.Println("Result: ", x)
}