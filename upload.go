package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"gopkg.in/gomail.v1"
)

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
	body := "Thank you for uploading your avatar. You can confirm your upload by clicking this link: " + link

	// Option 3: using Gomail
	msg := gomail.NewMessage()
	msg.SetHeader("From", from)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", title)
	msg.SetBody("text/html", body)

	config := tls.Config{}
	if *noTls {
		config.InsecureSkipVerify = true
	}
	mailer := gomail.NewCustomMailer(address, nil, gomail.SetTLSConfig(&config))
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
			if len(splitted) < 2 {
				log.Printf("Invalid confirmation file name: %v", filename)
				return "", "", errors.New("Internal error")
			}
			hash = splitted[1] // FIXME perform range check!
			return getUnconfirmedDir() + "/" + filename, hash, nil
		}
	}
	return "", "", errors.New("Confirmation period expired")
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

func uploadHandler(w http.ResponseWriter, r *http.Request, title string) {
	renderTemplate(w, "upload", map[string]string{})
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
	renderTemplate(w, "save", map[string]string{"Email": email})
}
