package main

import (
	"io/fs"
	"net/http"
)

func newFrontendHandler(frontend fs.FS) http.Handler {
	return newAssetHandler(frontend)
}
