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

type updateui struct {
	serverVersion string
	available     bool
	triggered     bool
	updatingText  string
}

func updateable() bool {
	return updateURL != "" && publicKeyString != ""
}

func updateCheck(ctx *ntcontext) {
	if !updateable() {
		return
	}
	log.Println("Checking for updates")
	bodybuf, err := fetchFile("version.txt")
	if err != nil {
		log.Println("Couldn't fetch version", err)
		return
	}
	body := strings.TrimSpace(string(bodybuf))

	ctx.update.serverVersion = body
	if ctx.update.serverVersion != version {
		ctx.update.available = true
	}

}

func update(ctx *ntcontext) {
	if !updateable() {
		return
	}
	sig, err := fetchFile("NoiseTorch_x64.tgz.sig")
	if err != nil {
		log.Println("Couldn't fetch signature", err)
		ctx.update.updatingText = "Update failed!"
		(*ctx.masterWindow).Changed()
		return
	}

	tgz, err := fetchFile("NoiseTorch_x64.tgz")
	if err != nil {
		log.Println("Couldn't fetch tgz", err)
		ctx.update.updatingText = "Update failed!"
		(*ctx.masterWindow).Changed()
		return
	}

	verified := ed25519.Verify(publickey(), tgz, sig)

	log.Printf("VERIFIED UPDATE: %t\n", verified)

	if !verified {
		log.Printf("SIGNATURE VERIFICATION FAILED, ABORTING UPDATE!\n")
		ctx.update.updatingText = "Update failed!"
		(*ctx.masterWindow).Changed()
		return
	}

	untar(bytes.NewReader(tgz), os.Getenv("HOME"))
	pkexecSetcapSelf()

	log.Printf("Update installed!\n")
	ctx.update.updatingText = "Update installed! (Restart NoiseTorch to apply)"
	(*ctx.masterWindow).Changed()
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
	if err != nil { // Should only happen when distributor ships an invalid public key
		log.Fatalf("Error while reading public key: %s\nContact the distribution '%s' about this error.\n", err, distribution)
		os.Exit(1)
	}
	return pub
}
