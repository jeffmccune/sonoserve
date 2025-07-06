//go:build ignore
// +build ignore

package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	// Build the Docusaurus website
	log.Println("Building Docusaurus website...")
	
	// Get the directory of this script (website directory)
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}
	
	// If we're not in the website directory, change to it
	if filepath.Base(wd) != "website" {
		wd = filepath.Join(wd, "website")
	}
	
	cmd := exec.Command("npm", "run", "build")
	cmd.Dir = wd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to build website: %v", err)
	}
	
	log.Println("Website built successfully!")
}