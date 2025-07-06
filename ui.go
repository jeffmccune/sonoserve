//go:generate go run website/generate.go

package main

import (
	"embed"
)

//go:embed website/build/*
var websiteFS embed.FS