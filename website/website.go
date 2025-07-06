//go:generate sh -c "rm -rf build && npm run build"

package website

import (
	"embed"
)

//go:embed all:build
var EmbeddedFS embed.FS