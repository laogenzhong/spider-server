package game

import (
	"strings"
	"testing"

	pb "spider-server/gen/spider/api"
)

func TestFormatRequestForLogUsesPlainKeyValueText(t *testing.T) {
	got := formatRequestForLog(&pb.SaveWeightRecordRequest{
		RecordDate: "2026-06-09",
		Weight:     705,
		Satiety:    8,
	})

	if got != "record_date=2026-06-09;weight=705;satiety=8" {
		t.Fatalf("formatRequestForLog = %q", got)
	}
	if strings.Contains(got, `"`) || strings.Contains(got, "{") {
		t.Fatalf("formatRequestForLog should not use JSON quoting: %q", got)
	}
}
