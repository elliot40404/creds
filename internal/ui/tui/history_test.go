package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/elliot40404/creds/internal/vault"
)

func withHistory() vault.Entry {
	at := func(d int) time.Time { return time.Date(2026, 1, d, 10, 30, 0, 0, time.UTC) }
	return vault.Entry{
		Path: "web/mail", Type: vault.TypeLogin, Username: "carol",
		Fields:  []vault.Field{{Name: "password", Value: "new", Secret: true}},
		Created: at(1), Updated: at(3), Machine: "laptop",
		History: []vault.Revision{
			{Machine: "desktop", At: at(2), Username: "bob", Fields: []vault.Field{{Name: "password", Value: "oldpw", Secret: true}}},
			{At: at(1), Username: "alice"},
		},
	}
}

func TestDetailShowsWhenAndWhere(t *testing.T) {
	d := newDetail(withHistory())
	v := formView(d)
	e := withHistory()
	for _, want := range []string{"created " + stamp(e.Created), "updated " + stamp(e.Updated), "on laptop"} {
		if !strings.Contains(v, want) {
			t.Fatalf("missing %q in %q", want, v)
		}
	}
}

func TestDetailOffersHistoryOnlyWhenThereIsSome(t *testing.T) {
	d := newDetail(withHistory())
	if !strings.Contains(keyList(d.keys()), "h") {
		t.Fatal("no h key with history")
	}
	_, msg := keys(d, ch('h'))
	if _, ok := msg.(historyMsg); !ok {
		t.Fatalf("msg %#v", msg)
	}
	plain := newDetail(vault.Entry{Path: "a", Type: vault.TypeNote})
	if strings.Contains(keyList(plain.keys()), "h") {
		t.Fatal("h offered without history")
	}
	if _, msg := keys(plain, ch('h')); msg != nil {
		t.Fatalf("msg %#v", msg)
	}
}

func keyList(keys []keyHelp) string {
	var out []string
	for _, k := range keys {
		out = append(out, k.key)
	}
	return strings.Join(out, " ")
}

func TestHistoryListsRevisionsNewestFirst(t *testing.T) {
	h := newHistory(withHistory())
	v := formView(h)
	e := withHistory()
	first := strings.Index(v, stamp(e.History[0].At))
	second := strings.Index(v, stamp(e.History[1].At))
	if first < 0 || second < 0 || first > second {
		t.Fatalf("order wrong in %q", v)
	}
	if !strings.Contains(v, "desktop") || !strings.Contains(v, "unknown machine") {
		t.Fatalf("machines missing in %q", v)
	}
}

func TestHistoryOpensARevision(t *testing.T) {
	h := newHistory(withHistory())
	_, msg := keys(h, enter)
	got, ok := msg.(revisionMsg)
	if !ok || got.index != 0 {
		t.Fatalf("msg %#v", msg)
	}
	_, msg = keys(h, down, enter)
	if got, ok := msg.(revisionMsg); !ok || got.index != 1 {
		t.Fatalf("msg %#v", msg)
	}
	_, msg = keys(h, ch('2'))
	if got, ok := msg.(revisionMsg); !ok || got.index != 1 {
		t.Fatalf("number pick %#v", msg)
	}
	if _, msg := keys(h, esc); msg == nil {
		t.Fatal("esc did not pop")
	}
}

func TestRevisionShowsOldValuesMaskedUntilRevealed(t *testing.T) {
	r := newRevision(withHistory(), 0)
	v := formView(r)
	if !strings.Contains(v, "bob") {
		t.Fatalf("old username missing in %q", v)
	}
	if strings.Contains(v, "oldpw") || !strings.Contains(v, masked) {
		t.Fatalf("old secret is not masked in %q", v)
	}
	if !strings.Contains(v, stamp(withHistory().History[0].At)) || !strings.Contains(v, "desktop") {
		t.Fatalf("stamp missing in %q", v)
	}
	s, msg := keys(r, down, ch('r'))
	if got, ok := msg.(revealMsg); !ok || got.path != "web/mail" || strings.Contains(formView(s), "oldpw") {
		t.Fatalf("reveal did not ask the backend: %#v", msg)
	}
	s, _ = s.Update(revealedMsg{entry: withHistory()})
	if !strings.Contains(formView(s), "oldpw") {
		t.Fatalf("reveal did not show the old secret: %q", formView(s))
	}
}

func TestRevisionDoesNotOfferEdits(t *testing.T) {
	r := newRevision(withHistory(), 0)
	list := keyList(r.keys())
	for _, k := range []string{"e", "d", "c", "y"} {
		if strings.Contains(" "+list+" ", " "+k+" ") {
			t.Fatalf("revision screen offers %q: %q", k, list)
		}
	}
	for _, k := range []tea.KeyPressMsg{ch('e'), ch('d'), ch('c'), ch('y')} {
		if _, msg := keys(r, k); msg != nil {
			t.Fatalf("key produced %#v", msg)
		}
	}
}
