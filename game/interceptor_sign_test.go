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

func TestPaywallAnalyticsServiceSkipsAuthentication(t *testing.T) {
	original := append([]string(nil), publicGRPCMethodPrefixes...)
	t.Cleanup(func() {
		publicGRPCMethodPrefixes = original
	})
	ConfigureAuth([]string{"/uc.", "/api.PaywallAnalyticsService/"})

	if !shouldSkipAuthInterceptor("/api.PaywallAnalyticsService/recordPaywallSession") {
		t.Fatal("paywall analytics must accept logged-out telemetry")
	}
	if shouldSkipAuthInterceptor("/api.VIPService/getVIPStatus") {
		t.Fatal("unrelated business services must remain authenticated")
	}
}

func TestFormatRequestForLogRedactsPaywallDeviceIdentifiers(t *testing.T) {
	got := formatRequestForLog(&pb.RecordPaywallSessionRequest{
		AnonymousId:    "11111111-1111-1111-1111-111111111111",
		DeviceUniqueId: "22222222-2222-2222-2222-222222222222",
	})
	if strings.Contains(got, "11111111") || strings.Contains(got, "22222222") {
		t.Fatalf("paywall device identifiers must be redacted: %q", got)
	}
	if strings.Count(got, "<redacted>") != 2 {
		t.Fatalf("paywall identifier redaction = %q", got)
	}
}
