package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"testing"
	"time"
)

func TestTimeNowStructured(t *testing.T) {
	m := NewManager()
	r := m.timeNowStructured(context.Background(), map[string]interface{}{})
	if r.Status != "success" {
		t.Fatalf("expected success, got %s (%s)", r.Status, r.Error)
	}
	if r.Tool != ToolTimeNow {
		t.Fatalf("expected tool %s, got %s", ToolTimeNow, r.Tool)
	}
	if r.Data == nil {
		t.Fatalf("expected data")
	}

	// Verify backward-compatible fields
	assertStringField(t, r.Data, "local_rfc3339")
	assertStringField(t, r.Data, "local_date")
	assertStringField(t, r.Data, "local_time")
	assertStringField(t, r.Data, "utc_rfc3339")
	assertStringField(t, r.Data, "timezone_name")
	assertStringField(t, r.Data, "timezone_offset")
	if _, ok := r.Data["unix_seconds"]; !ok {
		t.Fatal("missing unix_seconds")
	}

	// Verify new NTP fields
	assertStringField(t, r.Data, "time_source")
	ts := r.Data["time_source"].(string)
	if ts != "ntp" && ts != "system" {
		t.Fatalf("time_source must be 'ntp' or 'system', got %q", ts)
	}
	if ts == "ntp" {
		assertStringField(t, r.Data, "ntp_server")
	}
	if _, ok := r.Data["ntp_offset_ms"]; !ok {
		t.Fatal("missing ntp_offset_ms")
	}

	// Verify rich time fields
	assertStringField(t, r.Data, "day_of_week")
	assertStringField(t, r.Data, "day_of_week_cn")
	assertStringField(t, r.Data, "weekday_or_weekend")
	assertStringField(t, r.Data, "timestamp_human")
	if _, ok := r.Data["iso_week"]; !ok {
		t.Fatal("missing iso_week")
	}
	if _, ok := r.Data["day_of_year"]; !ok {
		t.Fatal("missing day_of_year")
	}
	if _, ok := r.Data["quarter"]; !ok {
		t.Fatal("missing quarter")
	}
	if _, ok := r.Data["is_leap_year"]; !ok {
		t.Fatal("missing is_leap_year")
	}

	// Validate weekday_or_weekend
	wowe := r.Data["weekday_or_weekend"].(string)
	if wowe != "weekday" && wowe != "weekend" {
		t.Fatalf("weekday_or_weekend must be 'weekday' or 'weekend', got %q", wowe)
	}

	// Validate quarter range
	q := r.Data["quarter"].(int)
	if q < 1 || q > 4 {
		t.Fatalf("quarter must be 1-4, got %d", q)
	}
}

func TestTimeNowNTPFallbackOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	m := NewManager()
	r := m.timeNowStructured(ctx, map[string]interface{}{})
	if r.Status != "success" {
		t.Fatalf("expected success even with canceled ctx, got %s", r.Status)
	}
	// Should fall back to system time
	if r.Data["time_source"] != "system" {
		t.Fatalf("expected system fallback on canceled ctx, got %v", r.Data["time_source"])
	}
}

func TestIsLeapYear(t *testing.T) {
	tests := []struct {
		year int
		want bool
	}{
		{2000, true},
		{1900, false},
		{2024, true},
		{2025, false},
		{2100, false},
		{2400, true},
	}
	for _, tt := range tests {
		if got := isLeapYear(tt.year); got != tt.want {
			t.Errorf("isLeapYear(%d) = %v, want %v", tt.year, got, tt.want)
		}
	}
}

func TestDayOfWeekCN(t *testing.T) {
	for _, d := range []time.Weekday{time.Sunday, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday} {
		if _, ok := dayOfWeekCN[d]; !ok {
			t.Errorf("dayOfWeekCN missing entry for %v", d)
		}
	}
}

func assertStringField(t *testing.T, data map[string]interface{}, key string) {
	t.Helper()
	v, ok := data[key]
	if !ok {
		t.Fatalf("missing field %q", key)
	}
	if _, ok := v.(string); !ok {
		t.Fatalf("field %q should be string, got %T", key, v)
	}
}
