package gitsync

import (
	"fmt"
	"strings"
	"testing"
)

const sampleID = "0123456789abcdef0123456789abcdef01234567"

func TestObjCacheTreeIsACopy(t *testing.T) {
	t.Parallel()
	var c objCache
	in := tree{"vault.json": "a"}
	c.putTree(sampleID, in)
	in["vault.json"] = "changed"
	got, ok := c.tree(sampleID)
	if !ok || got["vault.json"] != "a" {
		t.Fatalf("got %v %v", got, ok)
	}
	got["vault.json"] = "changed"
	if again, _ := c.tree(sampleID); again["vault.json"] != "a" {
		t.Fatalf("cache changed through a returned map: %v", again)
	}
}

func TestObjCacheSkipsNamesAndCaps(t *testing.T) {
	t.Parallel()
	var c objCache
	for _, rev := range []string{"HEAD", localRef, strings.ToUpper(sampleID), sampleID[:39]} {
		c.putTree(rev, tree{})
		if _, ok := c.tree(rev); ok {
			t.Fatalf("cached %q", rev)
		}
	}
	for i := range cacheLimit + 1 {
		c.putTree(fmt.Sprintf("%040x", i), tree{})
	}
	if len(c.trees) > cacheLimit {
		t.Fatalf("cache grew to %d", len(c.trees))
	}
}

func TestReadTreeCachedPerCommit(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	head := gitOut(t, p.a, "rev-parse", "HEAD")
	first, err := p.a.readTree(t.Context(), head)
	if err != nil {
		t.Fatal(err)
	}
	p.a.git.Dir = t.TempDir()
	second, err := p.a.readTree(t.Context(), head)
	if err != nil || len(second) != len(first) {
		t.Fatalf("second read %d %v, want %d from cache", len(second), err, len(first))
	}
}
