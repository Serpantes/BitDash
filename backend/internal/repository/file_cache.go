package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

type FileCache struct {
	root string
}

func NewFileCache(root string) (*FileCache, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &FileCache{root: root}, nil
}

func (c *FileCache) filePath(key string) string {
	name := sanitizeCacheFileName(key)
	return filepath.Join(c.root, name)
}

func sanitizeCacheFileName(key string) string {
	safe := strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', '\n', '\r', '\t':
			return '_'
		default:
			return r
		}
	}, key)
	if len(safe) > 0 && len(safe) <= 200 {
		return safe
	}
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:]) + ".bin"
}

func (c *FileCache) Get(key string) ([]byte, bool) {
	data, err := os.ReadFile(c.filePath(key))
	if err != nil {
		return nil, false
	}
	return data, true
}

func (c *FileCache) Set(key string, data []byte) error {
	path := c.filePath(key)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
