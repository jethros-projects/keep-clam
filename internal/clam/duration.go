package clam

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func ParseTimeout(value string) (time.Duration, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return 0, fmt.Errorf("timeout cannot be empty")
	}
	if strings.HasSuffix(value, "d") {
		days, err := strconv.ParseFloat(strings.TrimSuffix(value, "d"), 64)
		if err != nil || days <= 0 {
			return 0, fmt.Errorf("invalid timeout %q; use values like 4h, 90m, or 1d", value)
		}
		return time.Duration(days * float64(24*time.Hour)), nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid timeout %q; use values like 4h, 90m, or 1d", value)
	}
	return duration, nil
}

func FormatDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	duration = duration.Round(time.Second)
	days := duration / (24 * time.Hour)
	duration -= days * 24 * time.Hour
	hours := duration / time.Hour
	duration -= hours * time.Hour
	minutes := duration / time.Minute
	duration -= minutes * time.Minute
	seconds := duration / time.Second

	parts := make([]string, 0, 4)
	if days > 0 {
		parts = append(parts, plural(int64(days), "day"))
	}
	if hours > 0 {
		parts = append(parts, plural(int64(hours), "hour"))
	}
	if minutes > 0 {
		parts = append(parts, plural(int64(minutes), "minute"))
	}
	if seconds > 0 || len(parts) == 0 {
		parts = append(parts, plural(int64(seconds), "second"))
	}
	if len(parts) > 2 {
		parts = parts[:2]
	}
	return strings.Join(parts, " ")
}

func plural(n int64, unit string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, unit)
	}
	return fmt.Sprintf("%d %ss", n, unit)
}
