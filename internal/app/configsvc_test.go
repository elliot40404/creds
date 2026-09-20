package app

import (
	"testing"

	"github.com/elliot40404/creds/internal/config"
)

func fieldByKey(t *testing.T, fields []ConfigField, key string) ConfigField {
	t.Helper()
	for _, f := range fields {
		if f.Key == key {
			return f
		}
	}
	t.Fatalf("no field %q", key)
	return ConfigField{}
}

func TestConfigFieldsShowWhereBlankValuesComeFrom(t *testing.T) {
	s, _, _ := newService(t)
	fields := s.ConfigFields()

	machine := fieldByKey(t, fields, "vault.machine")
	if machine.Value != "" || machine.Source != SourceHostname || machine.Derived == "" {
		t.Fatalf("machine field %+v", machine)
	}
	if machine.Derived != config.DefaultMachine() {
		t.Fatalf("machine %q, want the hostname %q", machine.Derived, config.DefaultMachine())
	}

	for _, key := range []string{"sync.name", "sync.email"} {
		f := fieldByKey(t, fields, key)
		if f.Value != "" || f.Derived == "" {
			t.Fatalf("%s field %+v", key, f)
		}
		if f.Source != SourceGit && f.Source != sourceBuiltIn {
			t.Fatalf("%s source %q", key, f.Source)
		}
	}
}

func TestConfigFieldsDropTheSourceWhenSet(t *testing.T) {
	s, _, _ := newService(t)
	s.Config.Sync.Email = "mine@example.com"
	f := fieldByKey(t, s.ConfigFields(), "sync.email")
	if f.Value != "mine@example.com" || f.Derived != "" || f.Source != "" {
		t.Fatalf("field %+v", f)
	}
}
