package analytics

import "testing"

func TestParseSnapshotSchedule(t *testing.T) {
	tests := []struct {
		value                string
		hour, minute, second int
	}{
		{value: "23:59:30", hour: 23, minute: 59, second: 30},
		{value: "08:15", hour: 8, minute: 15, second: 0},
		{value: "invalid", hour: 23, minute: 59, second: 30},
	}
	for _, test := range tests {
		hour, minute, second := parseSnapshotSchedule(test.value)
		if hour != test.hour || minute != test.minute || second != test.second {
			t.Fatalf("parseSnapshotSchedule(%q) = %02d:%02d:%02d, want %02d:%02d:%02d", test.value, hour, minute, second, test.hour, test.minute, test.second)
		}
	}
}
