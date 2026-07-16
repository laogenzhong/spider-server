package analytics

import (
	"context"
	"strings"
	"time"

	applogger "spider-server/common/logger"
	mysqlmodel "spider-server/mysql/model"
)

func StartDailyActivitySnapshotter(ctx context.Context, schedule string) {
	hour, minute, second := parseSnapshotSchedule(schedule)
	go func() {
		now := time.Now().In(time.Local)
		todayRunAt := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, second, 0, time.Local)
		catchUpDay := now.AddDate(0, 0, -1)
		if !now.Before(todayRunAt) {
			catchUpDay = now
		}
		snapshotDay(catchUpDay)
		for {
			now = time.Now().In(time.Local)
			next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, second, 0, time.Local)
			if !next.After(now) {
				next = next.AddDate(0, 0, 1)
			}
			timer := time.NewTimer(time.Until(next))
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return
			case runAt := <-timer.C:
				snapshotDay(runAt.In(time.Local))
			}
		}
	}()
}

func snapshotDay(day time.Time) {
	rows, err := mysqlmodel.SnapshotDailyUserActivity(day, time.Now())
	if err != nil {
		applogger.Printf("daily user activity snapshot failed: day=%s err=%v", day.Format("2006-01-02"), err)
		return
	}
	applogger.Printf("daily user activity snapshot completed: day=%s rows=%d", day.Format("2006-01-02"), rows)
}

func parseSnapshotSchedule(value string) (int, int, int) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"15:04:05", "15:04"} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed.Hour(), parsed.Minute(), parsed.Second()
		}
	}
	return 23, 59, 30
}
