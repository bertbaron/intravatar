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
	dataDir  = flag.String("data", "data", "Path to data files relative to current working dir.")
	port     = flag.Int("port", 8080, "Webserver port number.")
	fallback = flag.String("fallback", "http://gravatar.com/avatar", "Fallback on this gravatar-compatible avatar service if no avatar is found, use 'none' for no fallback")
	dflt     = flag.String("default", "resources/mm", "Default avatar. Use 'fallback' to use the default of the fallback service, or 'fallback:<option>' to use a builtin default. For example: 'fallback:monsterid'. This is passes as 'd=monsterid' to the fallback service. See https://nl.gravatar.com/site/implement/images/.")
)

var (
	defaultImage = "resources/mm"
	fallbackUrl = ""
	fallbackDefault = ""
)

func writeFile(hash string, w io.Writer) error {
	filename := fmt.Sprintf("%s/%s", *dataDir, hash)
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		if fallbackUrl == "" {
			log.Printf("%s does not exist, falling back to default", filename)
			filename = defaultImage
		} else {
			log.Printf("%s does not exist, redirecting to %s", filename, fallbackUrl)
			options := ""
			if fallbackDefault != "" {
				options = fmt.Sprintf("?d=%s", fallbackDefault)
			}
			remote := fmt.Sprintf("%s/%s%s", fallbackUrl, hash, options)
			resp, err2 := http.Get(remote)
			if err2 != nil {
				log.Printf("Remote lookup of %s failed with error: %s", remote, err2)
			} else {
				if resp.StatusCode == 404 {
					log.Printf("Avatar not found on remote, falling back to default")
				} else {
					// TODO check for other status codes?
					log.Printf("Response: %v", resp)
					_, e := io.Copy(w, resp.Body)
					return e
				}
			}
			filename = "resources/mm"
		}
	}

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
	writeFile(title, w)
}

var validPath = regexp.MustCompile("^/(avatar)/([a-zA-Z0-9]+)$")

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Print("Handling request: ", r.URL)
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

var fallbackDefaultPattern = regexp.MustCompile("^fallback:([a-zA-Z]+)$")

func main() {
	iniflags.Parse()

	log.Printf("data dir = %s\n", *dataDir)
	address := fmt.Sprintf(":%d", *port)

	if *fallback == "none" {
		fallbackUrl = ""
	} else {
		fallbackUrl = *fallback
		log.Printf("Missig avatars will be redirected to %s", fallbackUrl)
	}
	
	if *dflt == "fallback" {
		log.Printf("Default image will be provided by the fallback service if configured")
		fallbackDefault = ""
	} else if builtin := fallbackDefaultPattern.FindStringSubmatch(*dflt); builtin != nil {
		fallbackDefault = builtin[1]
		log.Printf("Default image will be provided by the fallback service using '%s' if configured", fallbackDefault)
	} else {
		defaultImage = *dflt
		fallbackDefault = "404"
		log.Printf("Using %s as default image", defaultImage)
	}

	log.Printf("Listening on %s\n", address)
	http.HandleFunc("/avatar/", makeHandler(avatarHandler))
	x := http.ListenAndServe(address, nil)
	fmt.Println("Result: ", x)
}
