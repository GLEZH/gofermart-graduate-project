package internal_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDependencyRules(t *testing.T) {
	rules := map[string][]string{
		"domain":  {"/internal/usecase/", "/internal/adapter/", "/internal/presenter/", "/internal/app/", "/internal/config/"},
		"usecase": {"/internal/adapter/", "/internal/presenter/", "/internal/app/"},
	}
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return walkErr
		}
		parts := strings.Split(filepath.ToSlash(path), "/")
		if len(parts) < 2 {
			return nil
		}
		for layer, forbidden := range rules {
			if parts[0] != layer {
				continue
			}
			file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if parseErr != nil {
				return parseErr
			}
			for _, imported := range file.Imports {
				value, _ := strconv.Unquote(imported.Path.Value)
				for _, prefix := range forbidden {
					if strings.Contains(value, prefix) {
						t.Errorf("%s imports forbidden package %s", path, value)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
