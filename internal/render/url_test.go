package render

import (
	"reflect"
	"testing"
)

func TestBuildURL(t *testing.T) {
	cases := map[string]struct {
		c    Conn
		want string
	}{
		"full": {
			Conn{Host: "h", Port: "5432", Username: "bob", Password: tricky, Database: "app", Params: map[string]string{"sslmode": "require", "a b": "c&d"}},
			"postgresql://bob:p%40ss%3Aw%2Frd%27%22@h:5432/app?a+b=c%26d&sslmode=require",
		},
		"no user":     {Conn{Host: "h"}, "postgresql://h"},
		"only pass":   {Conn{Host: "h", Password: "x"}, "postgresql://:x@h"},
		"only user":   {Conn{Host: "h", Username: "u@x"}, "postgresql://u%40x@h"},
		"ipv6":        {Conn{Host: "::1"}, "postgresql://[::1]"},
		"ipv6 port":   {Conn{Host: "::1", Port: "1"}, "postgresql://[::1]:1"},
		"multi":       {Conn{Host: "a:1,b:2", Database: "d"}, "postgresql://a:1,b:2/d"},
		"no host":     {Conn{}, "postgresql:///"},
		"params only": {Conn{Host: "h", Params: map[string]string{"x": "1"}}, "postgresql://h/?x=1"},
		"db escaped":  {Conn{Host: "h", Database: "a b?c"}, "postgresql://h/a%20b%3Fc"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := buildURL("postgresql", tc.c); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestBuildURLRoundtrip(t *testing.T) {
	c := Conn{
		Scheme: "postgresql", Host: "h", Port: "5432", Username: "b o+b", Password: tricky,
		Database: "a/b c", Params: map[string]string{"k": "v=&%"},
	}
	got, err := Parse(Postgres, buildURL(c.Scheme, c))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, c) {
		t.Fatalf("got %+v, want %+v", got, c)
	}
}
