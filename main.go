package main

import (
	"bytes"
	"crypto/md5"
	"flag"
	"fmt"
	"github.com/nfnt/resize"
	"github.com/vharitonsky/iniflags"
	"html/template"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"github.com/oliamb/cutter"
)

// Options
var (
	dataDir = flag.String("data", "data", "Path to data files relative to current working dir.")
	port    = flag.Int("port", 8080, "Webserver port number.")
	remote  = flag.String("remote", "http://gravatar.com/avatar", "Use this gravatar-compatible avatar service if "+
		"no avatar is found, use 'none' for no remote.")
	dflt = flag.String("default", "remote:monsterid", "Default avatar. Use 'remote' to use the default of the remote\n"+
		"    service, or 'remote:<option>' to use a builtin default. For example: 'remote:monsterid'. This is passes as\n"+
		"    '?d=monsterid' to the remote service. See https://nl.gravatar.com/site/implement/images/.\n"+
		"    If no remote and no local default is configured, resources/mm is used as default.")
)

var (
	defaultImage  = "resources/mm"
	remoteUrl     = ""
	remoteDefault = ""
	templates     *template.Template
)

const (
	minSize = 8
	maxSize = 512
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

func avatar2Image(avatar *Avatar) (img image.Image, format string, err error) {
	return image.Decode(bytes.NewBuffer(avatar.data))
}

// scales the avatar (altering it!)
func image2Avatar(avatar *Avatar, img image.Image, format string) {
	b := new(bytes.Buffer)
	switch format {
	case "jpeg":
		jpeg.Encode(b, img, nil)
	case "gif":
		gif.Encode(b, img, nil)
	case "png":
		png.Encode(b, img)
	}
	avatar.data = b.Bytes()
	
}

// scales the avatar (altering it!)
func scale(avatar *Avatar, size int) error {
	img, format, err := avatar2Image(avatar)
	if err != nil {
		return err
	}
	actualSize := img.Bounds().Dx() // assume square
	if size == actualSize {
		return nil
	}
	log.Printf("Resizing img from %vx%v to %vx%v", actualSize, actualSize, size, size)
	resized := resize.Resize(uint(size), uint(size), img, resize.Bicubic)
	image2Avatar(avatar, resized, format)
	return nil
}

func min(x, y int) int {
	if x <= y {
		return x
	}
	return y
}

func cropAndScale(avatar *Avatar) error {
	img, format, err := avatar2Image(avatar)
	if err != nil {
		return err
	}
	x := img.Bounds().Dx()
	y := img.Bounds().Dy()
	size := min(x, y)
	if x != y {
		log.Printf("Cropping img from %vx%v to %vx%v", x, y, size, size)
		img, err = cutter.Crop(img, cutter.Config{
  			Width:  size,
  			Height: size,
  			Mode: cutter.Centered})
		if err != nil {
			return err
		}
	}
	if size <= maxSize {
		return nil
	}
	log.Printf("Resizing img from %vx%v to %vx%v", size, size, maxSize, maxSize)
	resized := resize.Resize(uint(maxSize), uint(maxSize), img, resize.Bicubic)
	image2Avatar(avatar, resized, format)
	return nil
}

func strictReadImage(reader io.Reader) (*Avatar, error) {
	b := new(bytes.Buffer)
	if _, e := io.Copy(b, reader); e != nil {
		return nil, e
	}
	return &Avatar{size: -1, data: b.Bytes()}, nil
}

func readImage(reader io.Reader) *Avatar {
	avatar, err := strictReadImage(reader)
	if err != nil {
		log.Printf("Could not read image", err)
		return nil
	}
	return avatar
}

func readFromFile(filename string, size int) *Avatar {
	file, err := os.Open(filename)
	if err != nil {
		log.Printf("Error reading file: %v", err)
		return nil
	}
	defer file.Close()
	avatar := readImage(file)
	err = scale(avatar, size)
	if err != nil {
		log.Printf("Could not scale image: %v", err)
		return nil // don't return the image, if we can't scale it it is probably corrupt
	}

	avatar.cacheControl = "max-age=300"
	if info, e := os.Stat(filename); e == nil {
		timestamp := info.ModTime().UTC()
		avatar.lastModified = timestamp.Format(http.TimeFormat)
	} else {
		avatar.lastModified = "Sat, 1 Jan 2000 12:00:00 GMT"
	}
	return avatar
}

func retrieveFromLocal(request Request) *Avatar {
	return readFromFile(createAvatarPath(request.hash), request.size)
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
	avatar := readImage(resp.Body)
	avatar.size = request.size // assume image is scaled by remote service
	avatar.cacheControl = resp.Header.Get("Cache-Control")
	avatar.lastModified = resp.Header.Get("Last-Modified")
	return avatar
}

func createAvatarPath(hash string) string {
	return fmt.Sprintf("%s/avatars/%s", *dataDir, hash)
}

func setHeaderField(w http.ResponseWriter, key string, value string) {
	if value != "" {
		w.Header().Set(key, value)
	}
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
			if size > maxSize {
				size = maxSize
			}
			if size < minSize {
				size = minSize
			}
		}
	}
	loadImage(Request{hash: title, size: size}, w, r)
}

func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
	err := templates.ExecuteTemplate(w, tmpl+".html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func uploadHandler(w http.ResponseWriter, r *http.Request, title string) {
	renderTemplate(w, "upload", map[string]string{})
}

func createHash(email string) string {
	h := md5.New()
	io.WriteString(h, strings.TrimSpace(strings.ToLower(email)))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func validateAndResize(file io.Reader) (*Avatar, error) {
	avatar, err := strictReadImage(file)
	if err != nil {
		return nil, err
	}
	err = cropAndScale(avatar)
	if err != nil {
		return nil, err
	}
	return avatar, nil
}

func renderSaveError(w http.ResponseWriter, message string, err error) {
	log.Printf("Error: %v (%v)", message, err)
	errMsg := fmt.Sprintf("%v", err)
	renderTemplate(w, "saveError", map[string]string{"Message": message, "Error": errMsg})
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	email := r.FormValue("email")
	log.Printf("Saving image for email address: %v", email)
	file, _, err := r.FormFile("image")
	if err != nil {
		renderSaveError(w, "Please chooce a file to upload", err)
		return
	}
	avatar, err2 := validateAndResize(file)
	if err2 != nil {
		renderSaveError(w, "Failed to read image file. Note that only jpeg, png and gif images are supported", err2)
		return
	}

	hash := createHash(email)
	filename := createAvatarPath(hash)

	f, err := os.Create(filename)
	if err != nil {
		renderSaveError(w, "Error while creating file", err)
		return
	}
	defer f.Close()
	b := bytes.NewBuffer(avatar.data)
	_, err = io.Copy(f, b)
	if err != nil {
		renderSaveError(w, "Failed to write file", err)
		return
	}

	renderTemplate(w, "save", map[string]string{"Avatar": fmt.Sprintf("/avatar/%s", hash)})
}

func mainHandler(w http.ResponseWriter, r *http.Request, title string) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}
	portName := ""
	if *port != 80 {
		portName = fmt.Sprintf(":%d", *port)
	}
	url := "http://" + hostname + portName + "/avatar/"
	renderTemplate(w, "index", map[string]string{"AvatarLink": url, "HostName": hostname})
}

// Creates a http request handler
// pattern is a regular expression that validates the URL. The first matching group is passes to the handler as title
func makeHandler(fn func(http.ResponseWriter, *http.Request, string), pattern string) http.HandlerFunc {
	regex := regexp.MustCompile(pattern)
	return func(w http.ResponseWriter, r *http.Request) {
		//		log.Printf("Request: %v", r)
		m := regex.FindStringSubmatch(r.URL.Path)
		if m == nil {
			log.Print("Invalid request: ", r.URL)
			http.NotFound(w, r)
			return
		}
		log.Printf("Handling request %v %v from %v", r.Method, r.URL, strings.Split(r.RemoteAddr, ":")[0])
		start := time.Now()
		fn(w, r, m[1])
		log.Printf("Handled request %v %v in %v", r.Method, r.URL, time.Since(start))
	}
}

var fallbackDefaultPattern = regexp.MustCompile("^remote:([a-zA-Z]+)$")

func initTemplates() {
	files, err := ioutil.ReadDir("resources/templates")
	if err != nil {
		log.Fatal(err)
	}
	var fileNames []string
	for _, file := range files {
		fileNames = append(fileNames, "resources/templates/"+file.Name())
	}
	templates = template.Must(template.ParseFiles(fileNames...))
}

func main() {
	iniflags.Parse()
	initTemplates()

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
	http.HandleFunc("/", makeHandler(mainHandler, "^/()$"))
	http.HandleFunc("/avatar/", makeHandler(avatarHandler, "^/avatar/([a-zA-Z0-9]+)$"))
	http.HandleFunc("/upload/", makeHandler(uploadHandler, "^/(upload)/$"))
	http.HandleFunc("/save/", makeHandler(saveHandler, "^/(save)/$"))
	x := http.ListenAndServe(address, nil)
	fmt.Println("Result: ", x)
}
