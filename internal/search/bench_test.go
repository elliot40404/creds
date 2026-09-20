package search

import (
	"strconv"
	"testing"

	"github.com/elliot40404/creds/internal/vault"
)

func benchItems(n int) []Summary {
	out := make([]Summary, n)
	for i := range out {
		s := strconv.Itoa(i)
		out[i] = Summary{Path: "team" + s + "/service/db" + s, Name: "db " + s, Type: vault.TypeDatabase, Host: "host" + s + ".example", Username: "user" + s, Tags: []string{"work", "prod"}}
	}
	return out
}

func BenchmarkKeystroke(b *testing.B) {
	x := NewIndex(benchItems(5000))
	b.ReportAllocs()
	for b.Loop() {
		x.Search("tdb4", SortPath)
	}
}
