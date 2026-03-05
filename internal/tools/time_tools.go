package tools

import (
	"context"
	"fmt"
	"time"
)

func (m *Manager) timeNowStructured(ctx context.Context, params map[string]interface{}) ToolResult {
	now := time.Now()
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

	data := map[string]interface{}{
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
	}

	display := fmt.Sprintf("%s (%s %s)", data["local_rfc3339"], zoneName, offStr)
	return ToolResult{Type: "tool_result", Tool: ToolTimeNow, Status: "success", Data: data, Display: display}
}

