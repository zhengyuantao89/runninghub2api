package runninghub

import "testing"

func TestIsNewBillingAndGift(t *testing.T) {
	ps := map[string]any{
		"code": 0,
		"data": map[string]any{
			"billingVersion": "NEW",
			"balancePoint":   30.0,
			"balanceUnit":    "30000",
		},
	}
	if !IsNewBilling(ps) {
		t.Fatal("expected NEW")
	}
	point, unit := PointRemain(ps)
	if point != "30" || unit != "30000" {
		t.Fatalf("point=%q unit=%q", point, unit)
	}
	w := Wallet{BillingVersion: BillingNew, RemainPoints: point, RemainCoins: "999"}
	if w.Spendable() != "30" {
		t.Fatalf("NEW spendable=%q", w.Spendable())
	}
	wZeroPts := Wallet{BillingVersion: BillingNew, RemainPoints: "0", RemainCoins: "522292"}
	if wZeroPts.Spendable() != "522292" {
		t.Fatalf("NEW with RH币 spendable=%q", wZeroPts.Spendable())
	}
	w2 := Wallet{BillingVersion: BillingOld, RemainCoins: "1100", RemainPoints: "30"}
	if w2.Spendable() != "1100" {
		t.Fatalf("OLD spendable=%q", w2.Spendable())
	}

	ws := map[string]any{
		"data": map[string]any{
			"dailyGift": map[string]any{
				"granted": true,
				"amount":  "30",
				"unit":    "POINT",
			},
		},
	}
	granted, amount, unit, ok := DailyGiftGranted(ws)
	if !ok || !granted || amount != "30" || unit != "POINT" {
		t.Fatalf("gift granted=%v amount=%q unit=%q ok=%v", granted, amount, unit, ok)
	}
}

func TestWalletFromAccountStatus(t *testing.T) {
	st := map[string]any{
		"code": 0,
		"data": map[string]any{
			"remainCoins":       "0",
			"remainPoint":       30.0,
			"currentTaskCounts": "0",
			"apiType":           "NORMAL",
		},
	}
	w := WalletFromAccountStatus(st)
	if w.BillingVersion != BillingNew {
		t.Fatalf("billing=%q", w.BillingVersion)
	}
	if w.RemainPoints != "30" {
		t.Fatalf("points=%q", w.RemainPoints)
	}
	if w.RemainCoins != "0" {
		t.Fatalf("coins=%q", w.RemainCoins)
	}
	if w.Spendable() != "30" {
		t.Fatalf("spendable=%q", w.Spendable())
	}

	old := WalletFromAccountStatus(map[string]any{
		"data": map[string]any{"remainCoins": "1100", "apiType": "NORMAL"},
	})
	if old.BillingVersion != BillingOld || old.RemainCoins != "1100" || old.RemainPoints != "" {
		t.Fatalf("old wallet %+v", old)
	}
}

func TestParseFreeQuota(t *testing.T) {
	ws := map[string]any{
		"data": map[string]any{
			"freeVideo": map[string]any{
				"total": 3.0, "used": 1.0, "remaining": 2.0,
			},
			"freeImage": map[string]any{
				"total": 3.0, "used": 0.0, "remaining": 3.0,
			},
		},
	}
	fq, ok := ParseFreeQuota(ws)
	if !ok {
		t.Fatal("expected ok")
	}
	if fq.ImageRemaining != 3 || fq.ImageTotal != 3 {
		t.Fatalf("image %+v", fq)
	}
	if fq.VideoRemaining != 2 || fq.VideoTotal != 3 {
		t.Fatalf("video %+v", fq)
	}
	if _, ok := ParseFreeQuota(map[string]any{"data": map[string]any{}}); ok {
		t.Fatal("empty should not ok")
	}
}
