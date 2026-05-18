package clover

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var assets embed.FS

// Assets returns the embedded static asset filesystem rooted at the contents of static/.
func Assets() http.FileSystem {
	sub, err := fs.Sub(assets, "static")
	if err != nil {
		panic(err)
	}
	return http.FS(sub)
}
