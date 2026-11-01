package server

import (
	"net/http"
	"path"
	"strings"
)

func assetHandler(root string, contentTypes map[string]string) http.Handler {
	files := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if contentType, ok := contentTypes[strings.ToLower(path.Ext(r.URL.Path))]; ok {
			w.Header().Set("Content-Type", contentType)
		}
		files.ServeHTTP(w, r)
	})
}
