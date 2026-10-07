package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// These paths must fail on the arguments alone, before any config or database is needed.
func TestUsageErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no command", nil, "usage"},
		{"unknown command", []string{"explode"}, `unknown command "explode"`},
		{"seed without scenario", []string{"seed"}, "--scenario"},
		{"seed unknown scenario", []string{"seed", "--scenario", "chaos"}, `unknown scenario "chaos"`},
		{"pact without subcommand", []string{"pact"}, "pact show"},
		{"pact show without id", []string{"pact", "show"}, "pact id"},
		{"pact show bad id", []string{"pact", "show", "nope"}, "not a UUID"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := run(context.Background(), tc.args, &out)
			if err == nil || !strings.Contains(err.Error()+out.String(), tc.want) {
				t.Fatalf("want an error mentioning %q, got err=%v out=%q", tc.want, err, out.String())
			}
		})
	}
}

func TestVersionPrintsWithoutConfig(t *testing.T) {
	var out bytes.Buffer
	if err := run(context.Background(), []string{"version"}, &out); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) == "" {
		t.Fatal("empty version")
	}
}
