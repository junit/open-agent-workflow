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
	if err := generateActiveAssets(root); err != nil {
		log.Fatal(err)
	}
}

func generateActiveAssets(root string) error {
	generators := [...]func(string) error{generatePolicyIntegrations}
	for _, generate := range generators {
		if err := generate(root); err != nil {
			return err
		}
	}
	return nil
}
