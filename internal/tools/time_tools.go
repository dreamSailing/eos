package tools

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/beevik/ntp"
)

// ntpServers defines the NTP server list with domestic servers first.
var ntpServers = []string{
	"ntp.aliyun.com",
	"time.windows.com",
	"cn.ntp.org.cn",
	"pool.ntp.org",
	"time.google.com",
	"time.cloudflare.com",
}

const (
	ntpSingleTimeout = 2 * time.Second
	ntpTotalTimeout  = 5 * time.Second
)

// ntpResult holds the result of an NTP query.
type ntpResult struct {
	time   time.Time
	server string
	offset time.Duration
}

// queryNTPTime tries NTP servers sequentially until one succeeds or total timeout.
func queryNTPTime(ctx context.Context) (*ntpResult, error) {
	deadline := time.After(ntpTotalTimeout)
	for _, server := range ntpServers {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline:
			return nil, fmt.Errorf("NTP total timeout exceeded")
		default:
		}

		resp, err := ntp.QueryWithOptions(server, ntp.QueryOptions{Timeout: ntpSingleTimeout})
		if err == nil && resp != nil {
			return &ntpResult{
				time:   time.Now().Add(resp.ClockOffset),
				server: server,
				offset: resp.ClockOffset,
			}, nil
		}
	}
	return nil, fmt.Errorf("all NTP servers failed")
}

// dayOfWeekCN maps English weekday to Chinese.
var dayOfWeekCN = map[time.Weekday]string{
	time.Sunday:    "星期日",
	time.Monday:    "星期一",
	time.Tuesday:   "星期二",
	time.Wednesday: "星期三",
	time.Thursday:  "星期四",
	time.Friday:    "星期五",
	time.Saturday:  "星期六",
}

func (m *Manager) timeNowStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	// Try NTP first, fall back to system time
	var (
		now        time.Time
		timeSource = "system"
		ntpServer  string
		ntpOffset  time.Duration
	)

	if result, err := queryNTPTime(ctx); err == nil {
		now = result.time
		timeSource = "ntp"
		ntpServer = result.server
		ntpOffset = result.offset
	} else {
		now = time.Now()
	}

	zoneName, offset := now.Zone()
	offSign := "+"
	off := offset
	if off < 0 {
		offSign = "-"
		off = -off
	}
	offH := off / 3600
	offM := (off % 3600) / 60
	offStr := fmt.Sprintf("%s%02d:%02d", offSign, offH, offM)

	// Compute rich fields
	_, week := now.ISOWeek()
	dayOfYear := now.YearDay()
	quarter := (int(now.Month()) - 1) / 3 + 1
	isLeapYear := isLeapYear(now.Year())
	weekday := now.Weekday()
	weekdayOrWeekend := "weekday"
	if weekday == time.Saturday || weekday == time.Sunday {
		weekdayOrWeekend = "weekend"
	}
	shortDayName := [...]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}[weekday]

	data := map[string]interface{}{
		// Original fields (backward compatible)
		"local_rfc3339":      now.Format(time.RFC3339),
		"local_rfc3339_nano": now.Format(time.RFC3339Nano),
		"local_date":         now.Format("2006-01-02"),
		"local_time":         now.Format("15:04:05"),
		"utc_rfc3339":        now.UTC().Format(time.RFC3339),
		"utc_rfc3339_nano":   now.UTC().Format(time.RFC3339Nano),
		"unix_seconds":       now.Unix(),
		"unix_millis":        now.UnixMilli(),
		"unix_nanos":         now.UnixNano(),
		"timezone_name":      zoneName,
		"timezone_offset":    offStr,
		"timezone_offset_s":  offset,
		"location":           now.Location().String(),

		// NTP source info
		"time_source":    timeSource,
		"ntp_server":     ntpServer,
		"ntp_offset_ms":  math.Round(ntpOffset.Seconds()*1000*100) / 100,

		// Rich time info
		"day_of_week":           weekday.String(),
		"day_of_week_cn":        dayOfWeekCN[weekday],
		"iso_week":              week,
		"day_of_year":           dayOfYear,
		"quarter":               quarter,
		"is_leap_year":          isLeapYear,
		"weekday_or_weekend":    weekdayOrWeekend,
		"timestamp_human":       fmt.Sprintf("%s %s %s", now.Format("2006-01-02"), shortDayName, now.Format("15:04:05")),
	}

	display := fmt.Sprintf("%s (%s %s) [%s]", data["local_rfc3339"], zoneName, offStr, timeSource)
	return ToolResult{Type: "tool_result", Tool: ToolTimeNow, Status: "success", Data: data, Display: display}
}

// isLeapYear returns true if the given year is a leap year.
func isLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}
