package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"github.com/vharitonsky/iniflags"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
)

var (
	dataDir = flag.String("data", "data", "Path to data files relative to current working dir.")
	port    = flag.Int("port", 8080, "Webserver port number.")
	remote  = flag.String("remote", "http://gravatar.com/avatar", "Use this gravatar-compatible avatar service if "+
		"no avatar is found, use 'none' for no remote.")
	dflt = flag.String("default", "resources/mm", "Default avatar. Use 'remote' to use the default of the remote "+
		"service, or 'remote:<option>' to use a builtin default. For example: 'remote:monsterid'. This is passes as "+
		"'?d=monsterid' to the remote service. See https://nl.gravatar.com/site/implement/images/.")
)

var (
	defaultImage  = "resources/mm"
	remoteUrl     = ""
	remoteDefault = ""
)

type Avatar struct {
	mimetype string
	size     int
	data     []byte
}

func createPath(hash string) string {
	return fmt.Sprintf("%s/%s", *dataDir, hash)
}

func loadImage(hash string, w io.Writer) error {
	filename := createPath(hash)
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		if remoteUrl == "" {
			log.Printf("%s does not exist, falling back to default", filename)
			filename = defaultImage
		} else {
			log.Printf("%s does not exist, redirecting to %s", filename, remoteUrl)
			options := ""
			if remoteDefault != "" {
				options = fmt.Sprintf("?d=%s", remoteDefault)
			}
			remote := fmt.Sprintf("%s/%s%s", remoteUrl, hash, options)
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
	loadImage(title, w)
}

var templates = template.Must(template.ParseFiles("resources/upload.html", "resources/view.html"))

func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
	err := templates.ExecuteTemplate(w, tmpl+".html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func uploadHandler(w http.ResponseWriter, r *http.Request, title string) {
	renderTemplate(w, "upload", map[string]string{"Title": "Upload your avatar"})
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	email := r.FormValue("email")
	log.Printf("Saving image for email address: %v", email)
	file, _, err := r.FormFile("datafile")
	if err != nil {
		log.Print("Error: ", err)
		fmt.Fprintf(w, "<p>Please chooce a file to upload</p>")
		return
	}

	h := md5.New()
	io.WriteString(h, email) // FIXME trim and toLowerCase!
	hash := fmt.Sprintf("%x", h.Sum(nil))
	filename := createPath(hash)

	f, err := os.Create(filename)
	if err != nil {
		log.Print("Error: ", err)
		fmt.Fprintf(w, "<p>Error while uploading file</p>")
		return
	}
	defer f.Close()
	io.Copy(f, file)

	renderTemplate(w, "view", map[string]string{"Avatar": fmt.Sprintf("/avatar/%s", hash)})
}

var validPath = regexp.MustCompile("^/(avatar|upload)/([a-zA-Z0-9]+)(/.*)?$")

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Request: %v", r)
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			log.Print("Invalid request: ", r.URL)
			http.NotFound(w, r)
			return
		}
		log.Print("Handling request: ", r.URL)
		fn(w, r, m[2])
	}
}

var fallbackDefaultPattern = regexp.MustCompile("^fallback:([a-zA-Z]+)$")

func main() {
	iniflags.Parse()

	log.Printf("data dir = %s\n", *dataDir)
	address := fmt.Sprintf(":%d", *port)

	if *remote == "none" {
		remoteUrl = ""
	} else {
		remoteUrl = *remote
		log.Printf("Missig avatars will be redirected to %s", remoteUrl)
	}

	if *dflt == "fallback" {
		log.Printf("Default image will be provided by the remote service if configured")
		remoteDefault = ""
	} else if builtin := fallbackDefaultPattern.FindStringSubmatch(*dflt); builtin != nil {
		remoteDefault = builtin[1]
		log.Printf("Default image will be provided by the remote service using '?d=%s' if a remote is configured", remoteDefault)
	} else {
		defaultImage = *dflt
		remoteDefault = "404"
		log.Printf("Using %s as default image", defaultImage)
	}

	log.Printf("Listening on %s\n", address)
	http.HandleFunc("/avatar/", makeHandler(avatarHandler))
	http.HandleFunc("/upload/form/", makeHandler(uploadHandler))
	http.HandleFunc("/upload/save/", makeHandler(saveHandler))
	x := http.ListenAndServe(address, nil)
	fmt.Println("Result: ", x)
}
