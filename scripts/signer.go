package main

import (
	"crypto"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ed25519"
)

func main() { //nolint
	var doGenerate bool
	flag.BoolVar(&doGenerate, "g", false, "Generate a keypair")

	var doPrintPub bool
	flag.BoolVar(&doPrintPub, "p", false, "Print the pub key")

	var doSign bool
	flag.BoolVar(&doSign, "s", false, "Sign the release tar")

	var publicKeyString string
	flag.StringVar(&publicKeyString, "k", "", "Public key to verify against (runs verifier if set)")

	var artifactFile string
	flag.StringVar(&artifactFile, "f", "", "Artifact file name and path that should be signed")

	flag.Parse()

	signatureFile := artifactFile + ".sig"

	if doGenerate {
		generateKeypair()
		os.Exit(0)
	}

	if doPrintPub {
		pub, _ := loadKeys()
		fmt.Printf("Public key: %s\n", base64.StdEncoding.EncodeToString(pub))
		os.Exit(0)
	}

	if doSign && artifactFile != "" {
		_, priv := loadKeys()

		file, err := os.ReadFile(artifactFile)
		if err != nil {
			panic(err)
		}

		sig, err := priv.Sign(nil, file, crypto.Hash(0))
		if err != nil {
			panic(err)
		}

		err = os.WriteFile(signatureFile, sig, 0640)
		if err != nil {
			panic(err)
		}
		os.Exit(0)
	}

	if publicKeyString != "" && artifactFile != "" && signatureFile != "" {
		pub, err := base64.StdEncoding.DecodeString(publicKeyString)
		if err != nil {
			panic(err)
		}

		file, err := os.ReadFile(artifactFile)
		if err != nil {
			panic(err)
		}

		sig, err := os.ReadFile(signatureFile)
		if err != nil {
			panic(err)
		}

		verified := ed25519.Verify(pub, file, sig)

		fmt.Printf("Verified %t\n", verified)
	}
}

func loadKeys() (ed25519.PublicKey, ed25519.PrivateKey) {
	seed, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config/noisetorch/private.key"))
	if err != nil {
		panic(err)
	}

	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)

	return pub, priv
}

func generateKeypair() {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic(err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".config/noisetorch/private.key"), priv.Seed(), 0600); err != nil {
		panic(err)
		os.Exit(2)
	}

	fmt.Printf("Private key generated and saved.\nPublic key: %s\n", base64.StdEncoding.EncodeToString(pub))
}
