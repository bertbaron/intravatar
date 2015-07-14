package main

import (
	"crypto/md5"
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
	"strings"
	"time"
)

// Options
var (
	dataDir = flag.String("data", "data", "Path to data files relative to current working dir.")
	host    = flag.String("host", "", "The dns name of this host. Defaults to the systems hostname")
	port    = flag.Int("port", 8080, "Webserver port number.")
	remote  = flag.String("remote", "https://gravatar.com/avatar", "Use this gravatar-compatible avatar service if "+
		"no avatar is found, use 'none' for no remote.")
	dflt = flag.String("default", "remote:monsterid", "Default avatar. Use 'remote' to use the default of the remote\n"+
		"    service, or 'remote:<option>' to use a builtin default. For example: 'remote:monsterid'. This is passes as\n"+
		"    '?d=monsterid' to the remote service. See https://nl.gravatar.com/site/implement/images/.\n"+
		"    If no remote and no local default is configured, resources/mm is used as default.")

	smtpHost = flag.String("smtp-host", "", "SMTP host used for email confirmation")
	smtpPort = flag.Int("smtp-port", 25, "SMTP port")
	sender   = flag.String("sender", "", "Senders email address")
	noTls    = flag.Bool("no-tls", false, "Disable tls encription for email, less secure! Can be useful if certificates of in-house mailhost are expired.")
)

var (
	defaultImage  = "resources/mm"
	remoteUrl     = ""
	remoteDefault = ""
	templates     *template.Template
)

const (
	minSize    = 8
	maxSize    = 512
	configFile = "config.ini"
)

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

func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
	err := templates.ExecuteTemplate(w, tmpl+".html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func createHash(email string) string {
	h := md5.New()
	io.WriteString(h, strings.TrimSpace(strings.ToLower(email)))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func getHostName() string {
	hostname := *host
	if hostname == "" {
		var err error
		hostname, err = os.Hostname()
		if err != nil {
			hostname = "localhost"
		}
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

func homeHandler(w http.ResponseWriter, r *http.Request, title string) {
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

// serves a single file
func serveSingle(pattern string, filename string) {
	http.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filename)
	})
}

func main() {
	if _, e := os.Stat(configFile); e == nil {
		log.Printf("Default configuration file %v", configFile)
		iniflags.SetConfigFile(configFile)
	}
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

	remoteFallbackPattern := regexp.MustCompile("^remote:([a-zA-Z]+)$")

	if *dflt == "fallback" {
		log.Printf("Default image will be provided by the remote service if configured")
		remoteDefault = ""
	} else if builtin := remoteFallbackPattern.FindStringSubmatch(*dflt); builtin != nil {
		remoteDefault = builtin[1]
		log.Printf("Default image will be provided by the remote service using '?d=%s' if a remote is configured", remoteDefault)
	} else {
		defaultImage = *dflt
		remoteDefault = "404"
		log.Printf("Using %s as default image", defaultImage)
	}

	log.Printf("Listening on %s\n", address)
	http.HandleFunc("/", makeHandler(homeHandler, "^/()$"))

	// Mandatory root-based resources
	serveSingle("/favicon.ico", "resources/favicon.ico")

	// Other static resources
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("resources/static/"))))

	// Application
	http.HandleFunc("/avatar/", makeHandler(avatarHandler, "^/avatar/([a-zA-Z0-9]+)$"))
	http.HandleFunc("/upload/", makeHandler(uploadHandler, "^/(upload)/$"))
	http.HandleFunc("/save/", makeHandler(saveHandler, "^/(save)/$"))
	http.HandleFunc("/confirm/", makeHandler(confirmHandler, "^/confirm/([a-zA-Z0-9]+)$"))
	x := http.ListenAndServe(address, nil)
	fmt.Println("Result: ", x)
}
