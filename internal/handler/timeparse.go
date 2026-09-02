package handler

import (
	"fmt"
	"time"
)

// timeParseDefault 缺省起始时间（一天前）。
var timeParseDefault = time.Now().Add(-24 * time.Hour)

// parseTimeRFC3339 解析 RFC3339；空串返回 fallback。
func parseTimeRFC3339(raw string, fallback time.Time) (time.Time, error) {
	if raw == "" {
		return fallback, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", raw, err)
	}
	return t, nil
}
