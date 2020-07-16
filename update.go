package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

var updateURL = "https://noisetorch.epicgamer.org"
var publicKeyString = "3mL+rBi4yBZ1wGimQ/oSQCjxELzgTh+673H4JdzQBOk="

type updateui struct {
	serverVersion string
	available     bool
	triggered     bool
	updatingText  string
}

func updateCheck(ui *uistate) {
	log.Println("Checking for updates")
	bodybuf, err := fetchFile("version.txt")
	if err != nil {
		log.Println("Couldn't fetch version", err)
		return
	}
	body := strings.TrimSpace(string(bodybuf))

	ui.update.serverVersion = body
	ui.update.available = true

}

func update(ui *uistate) {
	sig, err := fetchFile("NoiseTorch_x64.tgz.sig")
	if err != nil {
		log.Println("Couldn't fetch signature", err)
		ui.update.updatingText = "Update failed!"
		(*ui.masterWindow).Changed()
		return
	}

	tgz, err := fetchFile("NoiseTorch_x64.tgz")
	if err != nil {
		log.Println("Couldn't fetch tgz", err)
		ui.update.updatingText = "Update failed!"
		(*ui.masterWindow).Changed()
		return
	}

	verified := ed25519.Verify(publickey(), tgz, sig)

	log.Printf("VERIFIED UPDATE: %t\n", verified)

	if !verified {
		log.Printf("SIGNATURE VERIFICATION FAILED, ABORTING UPDATE!\n")
		ui.update.updatingText = "Update failed!"
		(*ui.masterWindow).Changed()
		return
	}

	untar(bytes.NewReader(tgz), os.Getenv("HOME"))

	log.Printf("Update installed!\n")
	ui.update.updatingText = "Update installed!"
	(*ui.masterWindow).Changed()
}

func fetchFile(file string) ([]byte, error) {
	resp, err := http.Get(updateURL + "/" + file)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received on 200 status code when fetching %s. Status: %s", file, resp.Status)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil

}

func publickey() []byte {
	pub, err := base64.StdEncoding.DecodeString(publicKeyString)
	if err != nil {
		panic(err) // it's hardcoded, we should never hit this, panic if we do
	}
	return pub
}
