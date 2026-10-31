package server

import (
	"net/http"
	"path"
	"strings"
)

var assetContentTypes = map[string]string{
	".pck":  "application/octet-stream",
	".wasm": "application/wasm",
}

func assetHandler(root string) http.Handler {
	files := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if contentType, ok := assetContentTypes[strings.ToLower(path.Ext(r.URL.Path))]; ok {
			w.Header().Set("Content-Type", contentType)
		}
		files.ServeHTTP(w, r)
	})
}
