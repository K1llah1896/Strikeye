package main

import (
	"reflect"
	"testing"
)

func TestResolveMode(t *testing.T) {
	cases := []struct {
		name    string
		program string
		args    []string
		mode    dispatchMode
		rest    []string
	}{
		{"bare binary opens the shell", "strikeye.exe", nil, modeDesktop, nil},
		{"gui flags stay with the shell", "strikeye.exe", []string{"--data-root", "x"}, modeDesktop, []string{"--data-root", "x"}},
		{"legacy agent flag", "strikeye.exe", []string{"--agent"}, modeDesktop, []string{"--agent"}},
		{"cli selector", "strikeye.exe", []string{"cli", "doctor"}, modeCLI, []string{"doctor"}},
		{"cli selector is case insensitive", "strikeye.exe", []string{"CLI", "export"}, modeCLI, []string{"export"}},
		{"agent selector", "strikeye.exe", []string{"agent", "--pipe", "p"}, modeAgent, []string{"--pipe", "p"}},
		{"renamed cli binary", "strikeye-cli.exe", []string{"doctor"}, modeCLI, []string{"doctor"}},
		{"renamed agent binary", "strikeye-agent.exe", []string{"--pipe", "p"}, modeAgent, []string{"--pipe", "p"}},
		{"dev build path", "/tmp/go-build/strikeye-cli", []string{"zones"}, modeCLI, []string{"zones"}},
		{"unknown name keeps the shell", "Strikeye-linux", nil, modeDesktop, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mode, rest := resolveMode(tc.program, tc.args)
			if mode != tc.mode {
				t.Fatalf("mode = %v, want %v", mode, tc.mode)
			}
			if !reflect.DeepEqual(rest, tc.rest) {
				t.Fatalf("rest = %v, want %v", rest, tc.rest)
			}
		})
	}
}
