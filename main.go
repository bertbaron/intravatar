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
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Options
var (
	dataDir = flag.String("data", "data", "Path to data files relative to current working dir.")
	port    = flag.Int("port", 8080, "Webserver port number.")
	webroot = flag.String("webroot", "", "The webroot of the service, defaults to http://localhost:<port>")
	logfile = flag.String("logfile", "", "Path to log file, if empty, the log will go to stderr of the process")

	remote = flag.String("remote", "https://gravatar.com/avatar", "Comma-separated list of gravatar-compatible avatar\n"+
		"    services to use if no avatar is found.")
	emailDomain = flag.String("emailDomain", "", "Comma-separated list of email domains\n"+
		"    allowed to change avatars. Empty value mean all domains are allowed.")
	dflt = flag.String("default", "remote:monsterid", "Default avatar. Use 'remote' to use the default of the (last) remote\n"+
		"    service, or 'remote:<option>' to use a builtin default. For example: 'remote:monsterid'. This is passed as\n"+
		"    '?d=monsterid' to the remote service. See https://nl.gravatar.com/site/implement/images/.\n"+
		"    If no remote and no local default is configured, resources/mm is used as default.")

	smtpHost     = flag.String("smtp-host", "", "SMTP host used for email confirmation, if not configured no confirmation emails will be required")
	smtpPort     = flag.Int("smtp-port", 25, "SMTP port")
	smtpUser     = flag.String("smtp-user", "", "SMTP user")
	smtpPassword = flag.String("smtp-password", "", "SMTP password")
	sender       = flag.String("sender", "", "Senders email address")
	noTLS        = flag.Bool("no-tls", false, "Disable tls encription for email, less secure! Can be useful if certificates of in-house mailhost are expired.")
	testMailAddr = flag.String("test-mail-addr", "", "If specified, sends a test email on startup to the given email address")
)

var (
	defaultImage  = "resources/mm"
	defaultFormat = "jpeg"
	remoteUrls    = []string{}
	emailDomains  = []string{}
	remoteDefault = ""
	templates     *template.Template
	storage       Storage
	localstorage  = NewFileStorage(".")
)

const (
	minSize    = 8
	maxSize    = 512
	configFile = "config.ini"
)

func exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	log.Fatalf("Error looking up directory %s", path)
	return false
}

func mkdir(path string) {
	if !exists(path) {
		log.Printf("Creating directory %s", path)
		os.Mkdir(path, 0700)
	}
}

func createAvatarPath(hash string) string {
	return filepath.Join("avatars", hash)
}

func createUnconfirmedAvatarPath(hash string, token string) string {
	return filepath.Join("unconfirmed", fmt.Sprintf("%s-%s", token, hash))
}

func getUnconfirmedDir() string {
	return "unconfirmed"
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

func getServiceURL() string {
	url := *webroot
	if url == "" {
		portName := ""
		if *port != 80 {
			portName = fmt.Sprintf(":%d", *port)
		}
		url = "http://localhost" + portName
	}
	return url + "/"
}

func homeHandler(w http.ResponseWriter, r *http.Request, title string) {
	url := getServiceURL() + "avatar/"
	renderTemplate(w, "index", map[string]string{"AvatarLink": url})
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

func initStorage() {
	storage = NewFileStorage(*dataDir)
	mkdir(*dataDir)
	mkdir(filepath.Join(*dataDir, "avatars"))
	mkdir(filepath.Join(*dataDir, "unconfirmed"))
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

	if *logfile != "" {
		file, err := os.OpenFile(*logfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("error opening file for logging: %v", err)
		}
		log.Printf("Logging will be redirected to %v", *logfile)
		defer file.Close()
		log.SetOutput(file)
	}

	if *smtpHost != "" && *sender == "" {
		if *sender == "" {
			log.Fatal("It is required to configure 'sender' when smtp host is not empty!")
		}
	}

	initTemplates()

	log.Printf("data dir = %s\n", *dataDir)
	address := fmt.Sprintf(":%d", *port)

	if *remote == "" {
		remoteUrls = []string{}
	} else {
		remoteUrls = strings.Split(*remote, ",")
		log.Printf("Missing avatars will be redirected to %s", remoteUrls)
	}
	if *emailDomain == "" {
		emailDomains = []string{}
	} else {
		emailDomains = strings.Split(*emailDomain, ",")
		for idx, domain := range emailDomains {
			emailDomains[idx] = strings.ToLower(domain)
		}
		log.Printf("Avatars will only be stored for email domains %s", emailDomains)
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

	if *testMailAddr != "" {
		if err := sendTestMail(*testMailAddr); err != nil {
			log.Fatalf("Failed to send test email to %s: %v", *testMailAddr, err)
		}
	}

	initStorage()

	log.Printf("Listening on %s\n", address)
	log.Printf("Service url: %s\n", getServiceURL())
	http.HandleFunc("/", makeHandler(homeHandler, "^/()$"))

	// Mandatory root-based resources
	serveSingle("/favicon.ico", "resources/favicon.ico")

	// Other static resources
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("resources/static/"))))

	// Application
	http.HandleFunc("/avatar/", makeHandler(avatarHandler, "^/avatar/([a-zA-Z0-9]+)(\\.[a-zA-Z0-9]+)?$"))
	http.HandleFunc("/upload/", makeHandler(uploadHandler, "^/(upload)/$"))
	http.HandleFunc("/save/", makeHandler(saveHandler, "^/(save)/$"))
	http.HandleFunc("/confirm/", makeHandler(confirmHandler, "^/confirm/([a-zA-Z0-9]+)$"))
	x := http.ListenAndServe(address, nil)
	fmt.Println("Result: ", x)
}
