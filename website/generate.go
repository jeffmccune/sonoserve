//go:build ignore
// +build ignore

package main

import (
	"log"
	"os"
	"os/exec"
)

func main() {
	// Build the Docusaurus website
	log.Println("Building Docusaurus website...")
	
	cmd := exec.Command("npm", "run", "build")
	cmd.Dir = "."
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to build website: %v", err)
	}
	
	log.Println("Website built successfully!")
}