package main

import (
	"bytes"
	"crypto/md5"
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/vharitonsky/iniflags"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"gopkg.in/gomail.v1"
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

	smtpHost = flag.String("smtp-host", "", "SMTP host used for email confirmation")
	smtpPort = flag.Int("smtp-port", 25, "SMTP port")
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

type Request struct {
	hash string
	size int
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

func createUnconfirmedAvatarPath(hash string, token string) string {
	return fmt.Sprintf("%s/unconfirmed/%s-%s", *dataDir, token, hash)
}

func getUnconfirmedDir() string {
	return fmt.Sprintf("%s/unconfirmed", *dataDir)
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

func sendConfirmationEmail(email string, token string) error {
	address := fmt.Sprintf("%v:%v", *smtpHost, *smtpPort)
	log.Printf("Sending confiration email to %v with confirmation token %v", email, address, token)

	from := "developers@asset-control.com"
	to := email
	title := "Please confirm your avatar upload"
 
 	url := getServiceUrl() + "confirm/" + token
 	link := fmt.Sprintf("<a href=\"%s\">%s</a>", url, url)
	body := "Thank you for uploading your avatar. You can confirm your upload by clicking this link: " + link;
 
	// Option 3: using Gomail
    msg := gomail.NewMessage()
    msg.SetHeader("From", from)
    msg.SetHeader("To", to)
    msg.SetHeader("Subject", title)
    msg.SetBody("text/html", body)

	// FIXME add option to skip tls
    mailer := gomail.NewCustomMailer(address, nil, gomail.SetTLSConfig(&tls.Config{InsecureSkipVerify: true}))
    if err := mailer.Send(msg); err != nil {
        panic(err)
    }	
	return nil
}

func createToken() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func renderSaveError(w http.ResponseWriter, message string, err error) {
	log.Printf("Error: %v (%v)", message, err)
	errMsg := fmt.Sprintf("%v", err)
	renderTemplate(w, "saveError", map[string]string{"Message": message, "Error": errMsg})
}

func getConfirmationFile(token string) (filepath string, hash string, err error) {
	files, err := ioutil.ReadDir(getUnconfirmedDir())
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range files {
		filename := file.Name()
		if strings.HasPrefix(filename, token) {
			splitted := strings.Split(filename, "-")
			hash = splitted[1] // FIXME perform range check!
			return getUnconfirmedDir() + "/" + filename, hash, nil
		}
	}
	return "", "", errors.New("Confirmation expired")
}

func confirm(w http.ResponseWriter, r *http.Request, token string) {
	log.Printf("Confirming uploaded avatar with token %v", token)	
	filepath, hash, err := getConfirmationFile(token)
	log.Printf("Found confirmation file %v (hash=%v)", filepath, hash)
	if err != nil {
		renderSaveError(w, "Error confirming upload", err)
		return
	}
	err = os.Rename(filepath, createAvatarPath(hash))
	if err != nil {
		renderSaveError(w, "Error confirming upload", err)
		return
	}

	// cache breaker to force website to reload the avatar
	ns := time.Now().UnixNano()
	uniq := fmt.Sprintf("%d", ns)
	
	// thank you for uploading your avatar...
	renderTemplate(w, "confirm", map[string]string{"Avatar": fmt.Sprintf("/avatar/%s", hash), "Uniq": uniq})
}

func confirmHandler(w http.ResponseWriter, r *http.Request, token string) {
	confirm(w, r, token)
}

func saveHandler(w http.ResponseWriter, r *http.Request, ignored string) {
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

	token, err3 := createToken()
	if err3 != nil {
		renderSaveError(w, "Failed to generate random token", err3)
		return
	}
	hash := createHash(email)
	filename := createUnconfirmedAvatarPath(hash, token)

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
	
	if *smtpHost == "" {
		// skip e-mail confirmation
		confirm(w, r, token)
		return
	}
	
	err = sendConfirmationEmail(email, token)
	if err != nil {
		renderSaveError(w, "Failed to send confirmation email", err)
		return
	}
	// a confirmation email has ben send...
	renderTemplate(w, "save", map[string]string{"Email" : email})
}

func getHostName() string {
	// FIXME hostname should be configurable
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}
	return hostname
}

func getServiceUrl() string {
	portName := ""
	if *port != 80 {
		portName = fmt.Sprintf(":%d", *port)
	}
	return "http://" + getHostName() + portName + "/"
}

func mainHandler(w http.ResponseWriter, r *http.Request, title string) {
	url := getServiceUrl() + "avatar/"
	renderTemplate(w, "index", map[string]string{"AvatarLink": url, "HostName": getHostName()})
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
	http.HandleFunc("/confirm/", makeHandler(confirmHandler, "^/confirm/([a-zA-Z0-9]+)$"))
	x := http.ListenAndServe(address, nil)
	fmt.Println("Result: ", x)
}
