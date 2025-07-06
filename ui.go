//go:generate go run website/generate.go

package main

import (
	"embed"
)

//go:embed all:website/build
var websiteFS embed.FS