package mysqlmodel

import "testing"

func TestPaywallSessionStatusNeverDowngrades(t *testing.T) {
	if paywallSessionStatusRank(PaywallSessionStatusDefault) >= paywallSessionStatusRank(PaywallSessionStatusCancelled) {
		t.Fatal("default status must rank below cancelled")
	}
	if paywallSessionStatusRank(PaywallSessionStatusCancelled) >= paywallSessionStatusRank(PaywallSessionStatusPurchased) {
		t.Fatal("cancelled status must rank below purchased")
	}
	if normalizePaywallSessionStatus("unknown") != PaywallSessionStatusDefault {
		t.Fatal("unknown status must normalize to default")
	}
}
