//strider:ignore-file cognitive-complexity,unchecked-type-assertion
package formatter

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/mod/modfile"
)

type modulePathCache struct {
	entries sync.Map
}

type cachedModulePath struct {
	path     string
	identity string
}

func (c *modulePathCache) find(filename string) string {
	return c.findInfo(filename).path
}

func (c *modulePathCache) findInfo(filename string) cachedModulePath {
	if filename == "" || strings.HasPrefix(filename, "<") {
		return cachedModulePath{}
	}
	directory, err := filepath.Abs(filepath.Dir(filename))
	if err != nil {
		return cachedModulePath{}
	}
	visited := []string{}
	for {
		if cached, ok := c.entries.Load(directory); ok {
			info := cached.(cachedModulePath)
			c.store(visited, info)
			return info
		}
		visited = append(visited, directory)
		if info, found := moduleInfoIn(directory); found {
			c.store(visited, info)
			return info
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			c.store(visited, cachedModulePath{})
			return cachedModulePath{}
		}
		directory = parent
	}
}

func (c *modulePathCache) store(directories []string, entry cachedModulePath) {
	for _, directory := range directories {
		c.entries.LoadOrStore(directory, entry)
	}
}

func modulePathIn(directory string) (string, bool) {
	info, found := moduleInfoIn(directory)
	return info.path, found
}

func moduleInfoIn(directory string) (cachedModulePath, bool) {
	content, err := os.ReadFile(filepath.Join(directory, "go.mod"))
	if err != nil {
		return cachedModulePath{}, false
	}
	digest := sha256.Sum256(content)
	return cachedModulePath{
		path:     modfile.ModulePath(content),
		identity: hex.EncodeToString(digest[:]),
	}, true
}
