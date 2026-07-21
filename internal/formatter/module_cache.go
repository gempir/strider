package formatter

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type modulePathCache struct {
	entries sync.Map
}

type cachedModulePath struct {
	path string
}

func (c *modulePathCache) find(filename string) string {
	if filename == "" || strings.HasPrefix(filename, "<") {
		return ""
	}
	directory, err := filepath.Abs(filepath.Dir(filename))
	if err != nil {
		return ""
	}
	visited := []string{}
	for {
		if cached, ok := c.entries.Load(directory); ok {
			path := cached.(cachedModulePath).path
			c.store(visited, path)
			return path
		}
		visited = append(visited, directory)
		if path, found := modulePathIn(directory); found {
			c.store(visited, path)
			return path
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			c.store(visited, "")
			return ""
		}
		directory = parent
	}
}

func (c *modulePathCache) store(directories []string, path string) {
	entry := cachedModulePath{
		path: path,
	}
	for _, directory := range directories {
		c.entries.LoadOrStore(directory, entry)
	}
}

func modulePathIn(directory string) (string, bool) {
	content, err := os.ReadFile(filepath.Join(directory, "go.mod"))
	if err != nil {
		return "", false
	}
	for len(content) != 0 {
		lineEnd := bytes.IndexByte(content, '\n')
		if lineEnd < 0 {
			lineEnd = len(content)
		}
		fields := bytes.Fields(content[:lineEnd])
		if len(fields) == 2 && bytes.Equal(fields[0], []byte("module")) {
			return string(fields[1]), true
		}
		if lineEnd == len(content) {
			break
		}
		content = content[lineEnd+1:]
	}
	return "", true
}
