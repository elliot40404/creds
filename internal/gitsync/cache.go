package gitsync

import (
	"maps"
	"slices"
	"sync"
)

const (
	cacheLimit   = 16
	blobLimit    = 32
	blobMaxBytes = 32 << 10
)

type objCache struct {
	mu    sync.Mutex
	trees map[string]tree
	blobs map[string][]byte
}

func (c *objCache) tree(sha string) (tree, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.trees[sha]
	return maps.Clone(t), ok
}

func (c *objCache) putTree(sha string, t tree) {
	if !isObjectID(sha) {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.trees == nil || len(c.trees) >= cacheLimit {
		c.trees = map[string]tree{}
	}
	c.trees[sha] = maps.Clone(t)
}

func (c *objCache) blob(sha string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	b, ok := c.blobs[sha]
	return slices.Clone(b), ok
}

func (c *objCache) putBlob(sha string, b []byte) {
	if !isObjectID(sha) || len(b) > blobMaxBytes {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.blobs == nil || len(c.blobs) >= blobLimit {
		c.blobs = map[string][]byte{}
	}
	c.blobs[sha] = slices.Clone(b)
}

func isObjectID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
