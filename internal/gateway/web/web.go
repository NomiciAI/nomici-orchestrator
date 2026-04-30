package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dist
var dist embed.FS

func Handler() http.Handler {
	subtree, err := fs.Sub(dist, "dist")
	if err != nil {
		return http.NotFoundHandler()
	}
	return http.FileServer(http.FS(subtree))
}
