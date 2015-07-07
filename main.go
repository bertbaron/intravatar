package main

import (
	"bytes"
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
	"strconv"
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
	size int
	data []byte
	// below are used in header fields
	cacheControl string
	lastModified string
}

type Request struct {
	hash string
	size int
}

func readImage(reader io.Reader, size int) *Avatar {
	b := new(bytes.Buffer)
	if _, e := io.Copy(b, reader); e != nil {
		log.Printf("Could not read image", e)
		return nil
	}
	// FIXME scale if necessary
	return &Avatar{size: size, data: b.Bytes()}
}

func readFromFile(filename string, size int) *Avatar {
	file, err := os.Open(filename)
	if err != nil {
		log.Printf("Error reading file: %v", err)
		return nil
	}
	defer file.Close()
	avatar := readImage(file, size)
	avatar.cacheControl = "300"
	if info, e := os.Stat(filename); e == nil {
		timestamp := info.ModTime().UTC()
		avatar.lastModified = timestamp.Format(http.TimeFormat) 
	} else {
		avatar.lastModified = "Sat, 1 Jan 2000 12:00:00 GMT"
	}
	return avatar
}

func retrieveFromLocal(request Request) *Avatar {
	return readFromFile(createPath(request.hash), request.size)
}

// Retrieves the avatar from the remote service, returning nil if there is no avatar or it could not be retrieved
func retrieveFromRemote(request Request) *Avatar {
	if remoteUrl == "" {
		return nil
	}
	options := fmt.Sprintf("s=%d", request.size)
	if remoteDefault != "" {
		options += "&d=" + remoteDefault
	}
	remote := remoteUrl + "/" + request.hash + "?" + options
	log.Printf("Retrieving from: %s", remote)
	resp, err2 := http.Get(remote)
	if err2 != nil {
		log.Printf("Remote lookup of %s failed with error: %s", remote, err2)
		return nil
	}
	if resp.StatusCode == 404 {
		log.Printf("Avatar not found on remote")
		return nil
	}
	avatar := readImage(resp.Body, request.size)
	avatar.cacheControl = resp.Header.Get("Cache-Control")
	avatar.lastModified = resp.Header.Get("Last-Modified")
	return avatar
}

func createPath(hash string) string {
	return fmt.Sprintf("%s/%s", *dataDir, hash)
}

func setHeaderField(w http.ResponseWriter, key string, value string) {
	if value != "" { w.Header().Set(key, value) }
}

func writeAvatarResult(w http.ResponseWriter, avatar *Avatar) {
	setHeaderField(w, "Last-Modified", avatar.lastModified)
	setHeaderField(w, "Cache-Control", avatar.cacheControl)
	b := bytes.NewBuffer(avatar.data)
	_, err := io.Copy(w, b)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func retrieveImage(request Request, w http.ResponseWriter, r *http.Request) *Avatar {
	avatar := retrieveFromLocal(request)
	if avatar == nil {
		avatar = retrieveFromRemote(request)
	}
	if avatar == nil {
		avatar = readFromFile(defaultImage, request.size)
	}
	if avatar == nil {
		avatar = readFromFile("resources/mm", request.size)
	}
	return avatar
}

func loadImage(request Request, w http.ResponseWriter, r *http.Request) {
	log.Printf("Loading image: %v", request)
	avatar := retrieveImage(request, w, r)
	if avatar == nil {
		http.NotFound(w, r)
	} else {
		writeAvatarResult(w, avatar)
	}
}

func avatarHandler(w http.ResponseWriter, r *http.Request, title string) {
	r.ParseForm()
	sizeParam := r.FormValue("s")
	size := 80
	if sizeParam != "" {
		if s, err := strconv.Atoi(sizeParam); err == nil {
			size = s
			if size > 512 {
				size = 512
			}
			if size < 8 {
				size = 8
			}
		}
	}
	loadImage(Request{hash: title, size: size}, w, r)
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
		//		log.Printf("Request: %v", r)
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

var fallbackDefaultPattern = regexp.MustCompile("^remote:([a-zA-Z]+)$")

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
