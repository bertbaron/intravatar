package main

import (
	"flag"
	"fmt"
	"github.com/vharitonsky/iniflags"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
)

var (
	dataDir = flag.String("data", "data", "Path to data files relative to current working dir.")
	port    = flag.Int("port", 8080, "Webserver port number.")
)

func writeFile(hash string, w io.Writer) error {
	filename := fmt.Sprintf("%s/%s", *dataDir, hash)
    
	file, err := os.Open(filename)
	if err != nil {
		log.Printf("Error reading file: %v", err)
		return err
	}

	defer file.Close()
	
	_, e := io.Copy(w, file)
	return e
}

func avatarHandler(w http.ResponseWriter, r *http.Request, title string) {
	log.Print("Handling avatar request", title)
	writeFile(title, w)
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
	log.Printf("data dir = %s\n", *dataDir)
	address := fmt.Sprintf(":%d", *port)
	log.Printf("Listening on %s\n", address)

	http.HandleFunc("/avatar/", makeHandler(avatarHandler))
	x := http.ListenAndServe(address, nil)
	fmt.Println("Result: ", x)
}
