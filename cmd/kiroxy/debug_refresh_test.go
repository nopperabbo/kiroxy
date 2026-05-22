package main

import "testing"

func TestRedactToken(t *testing.T) {
	tests := []struct {
		name  string
		input string
		n     int
		want  string
	}{
		{"short_string", "abc", 4, "***"},
		{"exact_boundary", "12345678901", 4, "***********"}, // len <= n*2+3
		{"normal", "abcdefghijklmnop", 4, "abcd"},
		{"empty", "", 4, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactToken(tt.input, tt.n)
			if got != tt.want {
				t.Errorf("redactToken(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
			}
		})
	}
}

func TestRedactTokenTail(t *testing.T) {
	tests := []struct {
		name  string
		input string
		n     int
		want  string
	}{
		{"short_string", "abc", 4, ""},
		{"exact_boundary", "12345678901", 4, ""}, // len <= n*2+3
		{"normal", "abcdefghijklmnop", 4, "mnop"},
		{"empty", "", 4, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactTokenTail(tt.input, tt.n)
			if got != tt.want {
				t.Errorf("redactTokenTail(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
			}
		})
	}
}
