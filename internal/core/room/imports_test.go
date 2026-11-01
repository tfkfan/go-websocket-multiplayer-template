package room

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var forbiddenCoreImports = []string{
	"github.com/tfkfan/gogame/internal/game",
	"github.com/tfkfan/gogame/internal/server",
	"github.com/tfkfan/gogame/internal/config",
}

func TestCoreDoesNotDependOnGamePackages(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	coreRoot := filepath.Dir(workingDir)

	scanned := 0
	err = filepath.Walk(coreRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		scanned++
		for _, spec := range file.Imports {
			importPath, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr != nil {
				continue
			}
			for _, forbidden := range forbiddenCoreImports {
				if importPath == forbidden || strings.HasPrefix(importPath, forbidden+"/") {
					t.Errorf("%s imports forbidden package %s", path, importPath)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk core packages: %v", err)
	}
	if scanned == 0 {
		t.Fatal("no core sources were scanned")
	}
}
