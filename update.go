package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/blang/semver/v4"
)

type github_releases []struct {
	Assets    []interface{} `json:"assets"`
	AssetsURL string        `json:"assets_url"`
	Author    struct {
		AvatarURL         string `json:"avatar_url"`
		EventsURL         string `json:"events_url"`
		FollowersURL      string `json:"followers_url"`
		FollowingURL      string `json:"following_url"`
		GistsURL          string `json:"gists_url"`
		GravatarID        string `json:"gravatar_id"`
		HTMLURL           string `json:"html_url"`
		ID                int64  `json:"id"`
		Login             string `json:"login"`
		NodeID            string `json:"node_id"`
		OrganizationsURL  string `json:"organizations_url"`
		ReceivedEventsURL string `json:"received_events_url"`
		ReposURL          string `json:"repos_url"`
		SiteAdmin         bool   `json:"site_admin"`
		StarredURL        string `json:"starred_url"`
		SubscriptionsURL  string `json:"subscriptions_url"`
		Type              string `json:"type"`
		URL               string `json:"url"`
	} `json:"author"`
	Body        string `json:"body"`
	CreatedAt   string `json:"created_at"`
	Draft       bool   `json:"draft"`
	HTMLURL     string `json:"html_url"`
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	NodeID      string `json:"node_id"`
	Prerelease  bool   `json:"prerelease"`
	PublishedAt string `json:"published_at"`
	Reactions   struct {
		PlusOne    int64  `json:"+1"`
		MinusOne   int64  `json:"-1"`
		Confused   int64  `json:"confused"`
		Eyes       int64  `json:"eyes"`
		Heart      int64  `json:"heart"`
		Hooray     int64  `json:"hooray"`
		Laugh      int64  `json:"laugh"`
		Rocket     int64  `json:"rocket"`
		TotalCount int64  `json:"total_count"`
		URL        string `json:"url"`
	} `json:"reactions"`
	TagName         string `json:"tag_name"`
	TarballURL      string `json:"tarball_url"`
	TargetCommitish string `json:"target_commitish"`
	UploadURL       string `json:"upload_url"`
	URL             string `json:"url"`
	ZipballURL      string `json:"zipball_url"`
}

type updateui struct {
	serverVersion string
	available     bool
	triggered     bool
	updatingText  string
}

var latestRelease, releaseError = getLatestRelease()
var latestVersion, _ = semver.Make(latestRelease)
var currentVersion, _ = semver.Make(version)

func updateable() bool {
	return updateURL != "" && publicKeyString != "" && releaseError == nil
}

func updateCheck(ctx *ntcontext) {
	if !updateable() {
		return
	}
	log.Println("Checking for updates")

	ctx.update.serverVersion = latestRelease
	if currentVersion.Compare(latestVersion) == -1 {
		ctx.update.available = true
	}

}

func update(ctx *ntcontext) {
	if !updateable() {
		return
	}
	sig, err := fetchFile("NoiseTorch_x64_" + latestRelease + ".tgz.sig")
	if err != nil {
		log.Println("Couldn't fetch signature", err)
		ctx.update.updatingText = "Update failed!"
		(*ctx.masterWindow).Changed()
		return
	}

	tgz, err := fetchFile("NoiseTorch_x64_" + latestRelease + ".tgz")
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
	resp, err := http.Get(updateURL + "/" + latestRelease + "/" + file)
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

func getLatestRelease() (string, error) {
	url := "https://api.github.com/repos/noisetorch/NoiseTorch/releases?per_page=1&page=1"

	httpclient := http.Client{
		Timeout: time.Second * 2, // Timeout after 2 seconds
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Println("Could not create http requester", err)
		return "", err
	}

	req.Header.Set("User-Agent", "NoiseTorch/"+version)

	res, err := httpclient.Do(req)
	if err != nil {
		log.Println("Couldn't fetch latest release", err)
		// Return an empty string when the latest release is unknown
		return "", err
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}

	respBytes := []byte(body)

	var latest_release github_releases

	err = json.Unmarshal(respBytes, &latest_release)
	if err != nil {
		log.Println("Reading JSON for latest_release failed", err)
		// Return an empty string when the JSON is something unexpected, for example: when rate limited
		return "", err
	}

	return latest_release[0].TagName, nil
}
