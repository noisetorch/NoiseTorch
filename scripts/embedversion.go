package main

import (
	"os"
	"os/exec"
	"strings"
)

func main() {
	cmd := exec.Command("git", "describe", "--tags")
	ret, err := cmd.Output()

	if err != nil {
		panic("Couldn't read git tags to embed version number")
	}
	version := strings.TrimSpace(string(ret))

	out, _ := os.Create("version.go")
	defer out.Close()

	out.Write([]byte("package main \n\nvar version=\""))
	out.Write([]byte(version))
	out.Write([]byte("\"\n"))
}
