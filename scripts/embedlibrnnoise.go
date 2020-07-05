package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
)

func main() {
	b, err := ioutil.ReadFile("librnnoise_ladspa/bin/ladspa/librnnoise_ladspa.so")
	if err != nil {
		fmt.Printf("Couldn't read librnnoise_ladspa.so: %v\n", err)
		os.Exit(1)
	}
	out, _ := os.Create("librnnoise.go")
	defer out.Close()

	out.Write([]byte("package main \n\nvar libRNNoise = []byte{\n"))
	for i, c := range b {
		out.Write([]byte(strconv.Itoa(int(c))))
		out.Write([]byte(","))
		if i%32 == 0 && i != 0 {
			out.Write([]byte("\n"))
		}
	}
	out.Write([]byte("}\n"))
}
