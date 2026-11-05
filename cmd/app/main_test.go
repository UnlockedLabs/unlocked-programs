package main

import (
	"context"
	"strings"
	"testing"
)

// Each command refuses to start without what it needs, before it connects to
// anything.
func TestRun(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		env     map[string]string
		wantErr string
	}{
		{name: "no command", wantErr: "usage"},
		{name: "unknown command", args: []string{"nope"}, wantErr: "unknown command"},
		{
			name:    "serve needs core's token",
			args:    []string{"serve"},
			env:     map[string]string{"PROGRAMS_TOKEN": "", "PROVIDER_PLATFORMS_TOKEN": "t"},
			wantErr: "PROGRAMS_TOKEN",
		},
		{
			name:    "serve needs Provider Platforms' token",
			args:    []string{"serve"},
			env:     map[string]string{"PROGRAMS_TOKEN": "t", "PROVIDER_PLATFORMS_TOKEN": ""},
			wantErr: "PROVIDER_PLATFORMS_TOKEN",
		},
		{
			name:    "seed-dev runs only in dev",
			args:    []string{"seed-dev"},
			env:     map[string]string{"APP_ENV": "production"},
			wantErr: "APP_ENV=dev",
		},
		{
			name:    "migrate-reset runs only in dev",
			args:    []string{"migrate-reset"},
			env:     map[string]string{"APP_ENV": "production"},
			wantErr: "APP_ENV=dev",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			err := run(context.Background(), tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("run(%v) error = %v, want one containing %q", tt.args, err, tt.wantErr)
			}
		})
	}
}
