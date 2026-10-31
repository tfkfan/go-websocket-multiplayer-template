package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssetHandlerServesExport(t *testing.T) {
	root := t.TempDir()
	writeAsset(t, root, "index.html", "<!doctype html><title>Circles</title>")
	writeAsset(t, root, "index.wasm", "\x00asm")
	writeAsset(t, root, "index.pck", "GDPC")

	server := httptest.NewServer(assetHandler(root))
	defer server.Close()

	tests := []struct {
		path        string
		status      int
		contentType string
		body        string
	}{
		{path: "/", status: http.StatusOK, contentType: "text/html", body: "Circles"},
		{path: "/index.wasm", status: http.StatusOK, contentType: "application/wasm", body: "asm"},
		{path: "/index.pck", status: http.StatusOK, contentType: "application/octet-stream", body: "GDPC"},
		{path: "/missing.pck", status: http.StatusNotFound, contentType: "text/plain"},
	}

	for _, test := range tests {
		response, err := http.Get(server.URL + test.path)
		if err != nil {
			t.Fatalf("request %s failed: %v", test.path, err)
		}
		body, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil {
			t.Fatalf("read %s failed: %v", test.path, err)
		}
		if response.StatusCode != test.status {
			t.Fatalf("request %s status = %d, want %d", test.path, response.StatusCode, test.status)
		}
		if contentType := response.Header.Get("Content-Type"); !strings.HasPrefix(contentType, test.contentType) {
			t.Fatalf("request %s content type = %q, want prefix %q", test.path, contentType, test.contentType)
		}
		if test.body != "" && !strings.Contains(string(body), test.body) {
			t.Fatalf("request %s body = %q, want it to contain %q", test.path, body, test.body)
		}
	}
}

func writeAsset(t *testing.T, root string, name string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s failed: %v", name, err)
	}
}
