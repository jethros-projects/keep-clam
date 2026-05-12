package clam

import (
	"testing"
	"time"
)

func TestParseTimeout(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want time.Duration
	}{
		{name: "hours", in: "4h", want: 4 * time.Hour},
		{name: "minutes", in: "90m", want: 90 * time.Minute},
		{name: "days", in: "1.5d", want: 36 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimeout(tt.in)
			if err != nil {
				t.Fatalf("ParseTimeout(%q) returned error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("ParseTimeout(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseTimeoutRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"", "0", "-1h", "clam"} {
		if _, err := ParseTimeout(value); err == nil {
			t.Fatalf("ParseTimeout(%q) returned nil error", value)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	got := FormatDuration(25*time.Hour + 3*time.Minute)
	want := "1 day 1 hour"
	if got != want {
		t.Fatalf("FormatDuration() = %q, want %q", got, want)
	}
}
