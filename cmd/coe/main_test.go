package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseServeOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr bool
	}{
		{
			name: "default",
			args: nil,
			want: "",
		},
		{
			name: "debug override",
			args: []string{"--log-level", "debug"},
			want: "debug",
		},
		{
			name:    "invalid level",
			args:    []string{"--log-level", "trace"},
			wantErr: true,
		},
		{
			name:    "extra args",
			args:    []string{"extra"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseServeOptions(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("parseServeOptions() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseServeOptions() error = %v", err)
			}
			if got.LogLevel != tt.want {
				t.Fatalf("parseServeOptions().LogLevel = %q, want %q", got.LogLevel, tt.want)
			}
		})
	}
}

func TestPrintDoctorChecksUsesEnglishSummaryAndStatuses(t *testing.T) {
	t.Parallel()

	checks := []doctorCheck{
		{Name: "Config file", OK: true, Detail: "path=/tmp/coe/config.yaml"},
		{Name: "Fcitx module init", OK: false, Detail: "marker=missing", Problem: "module init marker is missing"},
	}

	var buf bytes.Buffer
	printDoctorChecks(&buf, checks)
	output := buf.String()

	for _, want := range []string{
		"Config file",
		"Fcitx module init",
		"OK",
		"FAIL",
		"Summary: issues found.",
		"- module init marker is missing",
		"\n      path=/tmp/coe/config.yaml",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("printDoctorChecks() missing %q in:\n%s", want, output)
		}
	}
}
