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
	// The v4 cutover has one active Host generator. Historical audit and
	// superseded schema assets are intentionally not regeneration inputs.
	generators := [...]func(string) error{generateCodexHost}
	for _, generate := range generators {
		if err := generate(root); err != nil {
			return err
		}
	}
	return nil
}
