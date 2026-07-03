package staticserve

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrUnsafePath = errors.New("unsafe static asset path")

type Resolver struct {
	root string
}

func NewResolver(root string) (Resolver, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return Resolver{}, fmt.Errorf("static root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return Resolver{}, err
	}
	return Resolver{root: filepath.Clean(abs)}, nil
}

func (resolver Resolver) Resolve(requestPath string) (string, error) {
	requestPath = strings.TrimSpace(requestPath)
	if requestPath == "" || requestPath == "/" {
		requestPath = "index.html"
	}
	if filepath.IsAbs(requestPath) {
		return "", ErrUnsafePath
	}
	clean := filepath.Clean(strings.TrimPrefix(requestPath, "/"))
	if clean == "." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || clean == ".." {
		return "", ErrUnsafePath
	}
	resolved := filepath.Join(resolver.root, clean)
	if !strings.HasPrefix(resolved, resolver.root+string(os.PathSeparator)) && resolved != resolver.root {
		return "", ErrUnsafePath
	}
	return resolved, nil
}

