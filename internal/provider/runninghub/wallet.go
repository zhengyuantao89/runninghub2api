package runninghub

import (
	"fmt"
	"strings"
)

const (
	BillingNew = "NEW"
	BillingOld = "OLD"
)

// Wallet keeps RH币 and 积分 separate.
type Wallet struct {
	BillingVersion    string
	RemainCoins       string // 旧体系 RH 币
	RemainPoints      string // 新体系积分
	BalanceUnit       string
	CurrentTaskCounts string
	APIType           string
	RawPoint          map[string]any
	RawAccountStatus  map[string]any
}

// Spendable returns a non-empty balance string for logging / legacy callers.
// Prefer the primary meter for the billing version, but fall back to the other
// so NEW accounts with RH币 (webapp) are not treated as empty.
func (w Wallet) Spendable() string {
	coins := strings.TrimSpace(w.RemainCoins)
	points := strings.TrimSpace(w.RemainPoints)
	if strings.EqualFold(w.BillingVersion, BillingNew) {
		if points != "" && points != "0" && points != "0.0" {
			return points
		}
		if coins != "" {
			return coins
		}
		return points
	}
	if coins != "" && coins != "0" && coins != "0.0" {
		return coins
	}
	if points != "" {
		return points
	}
	return coins
}

// IsNewBilling reports whether /uc/point/summary says billingVersion=NEW.
func IsNewBilling(pointSummary map[string]any) bool {
	return strings.EqualFold(BillingVersionOf(pointSummary), BillingNew)
}

func BillingVersionOf(pointSummary map[string]any) string {
	data, _ := pointSummary["data"].(map[string]any)
	if data == nil {
		return ""
	}
	return strings.TrimSpace(asString(data["billingVersion"]))
}

func PointRemain(pointSummary map[string]any) (point, unit string) {
	data, _ := pointSummary["data"].(map[string]any)
	if data == nil {
		return "", ""
	}
	point = strings.TrimSpace(asString(data["balancePoint"]))
	unit = strings.TrimSpace(asString(data["balanceUnit"]))
	return point, unit
}

func DailyGiftGranted(welfareSummary map[string]any) (granted bool, amount, unit string, ok bool) {
	data, _ := welfareSummary["data"].(map[string]any)
	if data == nil {
		return false, "", "", false
	}
	gift, _ := data["dailyGift"].(map[string]any)
	if gift == nil {
		return false, "", "", false
	}
	switch v := gift["granted"].(type) {
	case bool:
		granted = v
	case string:
		granted = strings.EqualFold(v, "true") || v == "1"
	default:
		granted = false
	}
	amount = strings.TrimSpace(asString(gift["amount"]))
	unit = strings.TrimSpace(asString(gift["unit"]))
	return granted, amount, unit, true
}

// FreeQuota is daily free image/video allowance from /api/welfare/summary.
type FreeQuota struct {
	ImageRemaining int
	ImageTotal     int
	VideoRemaining int
	VideoTotal     int
}

// ParseFreeQuota extracts freeImage/freeVideo remaining+total. ok=false if neither block present.
func ParseFreeQuota(welfareSummary map[string]any) (FreeQuota, bool) {
	var out FreeQuota
	data, _ := welfareSummary["data"].(map[string]any)
	if data == nil {
		return out, false
	}
	img, _ := data["freeImage"].(map[string]any)
	vid, _ := data["freeVideo"].(map[string]any)
	if img == nil && vid == nil {
		return out, false
	}
	if img != nil {
		out.ImageRemaining = nonNegInt(img["remaining"])
		out.ImageTotal = nonNegInt(img["total"])
	}
	if vid != nil {
		out.VideoRemaining = nonNegInt(vid["remaining"])
		out.VideoTotal = nonNegInt(vid["total"])
	}
	return out, true
}

func nonNegInt(v any) int {
	n := asInt(v)
	if n < 0 {
		return 0
	}
	return n
}

// FetchFreeQuota calls welfare/summary and parses free image/video quotas.
func (c *Client) FetchFreeQuota() (FreeQuota, error) {
	ws, err := c.WelfareSummary(false)
	if err != nil {
		return FreeQuota{}, err
	}
	fq, ok := ParseFreeQuota(ws)
	if !ok {
		return FreeQuota{}, fmt.Errorf("welfare/summary missing freeImage/freeVideo")
	}
	return fq, nil
}

// WalletFromAccountStatus maps /uc/openapi/accountStatus into Wallet.
// remainPoint is present for NEW billing (may be negative); remainCoins for RH币.
func WalletFromAccountStatus(st map[string]any) Wallet {
	var out Wallet
	out.RawAccountStatus = st
	data, _ := st["data"].(map[string]any)
	if data == nil {
		return out
	}
	if v, ok := data["remainCoins"]; ok && v != nil {
		out.RemainCoins = strings.TrimSpace(asString(v))
	}
	if v, ok := data["remainPoint"]; ok && v != nil {
		out.RemainPoints = strings.TrimSpace(asString(v))
		out.BillingVersion = BillingNew
	}
	if v, ok := data["currentTaskCounts"]; ok && v != nil {
		out.CurrentTaskCounts = strings.TrimSpace(fmt.Sprint(v))
	}
	if v, ok := data["apiType"]; ok && v != nil {
		out.APIType = strings.TrimSpace(fmt.Sprint(v))
	}
	if out.BillingVersion == "" {
		out.BillingVersion = BillingOld
	}
	return out
}

// FetchWallet prefers consumer apiKey /uc/openapi/accountStatus (no AT).
// Falls back to NEW /uc/point/summary only when apiKey is missing/failed and AT is set.
func (c *Client) FetchWallet(apiKey string) (Wallet, error) {
	key := strings.TrimSpace(apiKey)
	var keyErr error
	if key != "" {
		st, err := c.AccountStatus(key)
		if err == nil {
			return WalletFromAccountStatus(st), nil
		}
		keyErr = err
		if strings.TrimSpace(c.AccessToken) == "" {
			return Wallet{}, err
		}
	}

	if strings.TrimSpace(c.AccessToken) != "" {
		ps, err := c.PointSummary()
		if err != nil {
			if keyErr != nil {
				return Wallet{}, fmt.Errorf("apiKey wallet: %v; AT point/summary: %w", keyErr, err)
			}
			return Wallet{}, err
		}
		if IsNewBilling(ps) {
			point, unit := PointRemain(ps)
			return Wallet{
				BillingVersion: BillingNew,
				RemainPoints:   point,
				BalanceUnit:    unit,
				RawPoint:       ps,
			}, nil
		}
	}

	if keyErr != nil {
		return Wallet{}, keyErr
	}
	return Wallet{}, fmt.Errorf("wallet needs apiKey (preferred) or NEW point/summary via access_token")
}
