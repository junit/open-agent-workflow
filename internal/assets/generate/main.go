package main

import (
	"log"
	"os"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	if err := generateCodexHost(root); err != nil {
		log.Fatal(err)
	}
}
