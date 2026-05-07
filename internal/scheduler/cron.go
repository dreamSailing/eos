package scheduler

import (
	"fmt"
	"strings"
	"time"

	cronlib "github.com/robfig/cron/v3"
)

var cronParser = cronlib.NewParser(cronlib.Minute | cronlib.Hour | cronlib.Dom | cronlib.Month | cronlib.Dow)

func nextRun(expr string, from time.Time) (time.Time, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return time.Time{}, fmt.Errorf("cron required")
	}
	spec, err := cronParser.Parse(expr)
	if err != nil {
		return time.Time{}, err
	}
	return spec.Next(from), nil
}
