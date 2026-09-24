package testutil_test

import (
	"errors"
	"testing"

	"filippo.io/age/plugin"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/testutil"
)

func TestMain(m *testing.M) { testutil.Main(m) }

func sealed(t *testing.T) (*crypto.Identity, []byte, string) {
	t.Helper()
	idStr, recStr := testutil.Plugin(t)
	r, err := plugin.NewRecipient(recStr, &plugin.ClientUI{})
	if err != nil {
		t.Fatal(err)
	}
	id := testutil.Identity(t)
	data, err := crypto.SealIdentity(id, r)
	if err != nil {
		t.Fatal(err)
	}
	return id, data, idStr
}

func open(t *testing.T, data []byte, idStr string, ui *plugin.ClientUI) (*crypto.Identity, error) {
	t.Helper()
	pid, err := plugin.NewIdentity(idStr, ui)
	if err != nil {
		t.Fatal(err)
	}
	return crypto.OpenIdentity(data, pid)
}

func TestPluginRoundtrip(t *testing.T) {
	id, data, idStr := sealed(t)
	got, err := open(t, data, idStr, &plugin.ClientUI{})
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != id.String() {
		t.Fatal("identity mismatch")
	}
}

func TestPluginWrongKey(t *testing.T) {
	_, data, _ := sealed(t)
	other, _ := testutil.Plugin(t)
	if _, err := open(t, data, other, &plugin.ClientUI{}); !errors.Is(err, crypto.ErrWrongKey) {
		t.Fatalf("got %v", err)
	}
}

func TestPluginCancel(t *testing.T) {
	t.Setenv(testutil.PluginModeEnv, "cancel")
	_, data, idStr := sealed(t)
	_, err := open(t, data, idStr, &plugin.ClientUI{})
	if err == nil || errors.Is(err, crypto.ErrWrongKey) {
		t.Fatalf("got %v", err)
	}
}

func TestPluginPIN(t *testing.T) {
	t.Setenv(testutil.PluginModeEnv, "pin")
	_, data, idStr := sealed(t)
	var asked string
	ui := &plugin.ClientUI{RequestValue: func(_, prompt string, _ bool) (string, error) {
		asked = prompt
		return testutil.PluginPIN, nil
	}}
	if _, err := open(t, data, idStr, ui); err != nil {
		t.Fatal(err)
	}
	if asked != "PIN" {
		t.Fatalf("asked %q", asked)
	}
}
