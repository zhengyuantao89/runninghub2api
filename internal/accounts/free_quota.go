package accounts

import (
	"log"
	"strings"

	"github.com/trae/runninghub2api/internal/provider/runninghub"
	"github.com/trae/runninghub2api/internal/store"
)

// RefreshFreeQuota pulls /api/welfare/summary and persists free image/video remaining+total.
// Failures are logged and ignored so balance refresh / claim paths stay resilient.
func RefreshFreeQuota(st *store.Store, client *runninghub.Client, accountID int64) {
	if st == nil || client == nil || accountID <= 0 {
		return
	}
	if strings.TrimSpace(client.AccessToken) == "" {
		return
	}
	fq, err := client.FetchFreeQuota()
	if err != nil {
		log.Printf("[account %d] free quota refresh skipped: %v", accountID, err)
		return
	}
	if err := st.UpdateAccountFreeQuota(accountID, fq.ImageRemaining, fq.ImageTotal, fq.VideoRemaining, fq.VideoTotal); err != nil {
		log.Printf("[account %d] free quota save failed: %v", accountID, err)
		return
	}
	log.Printf("[account %d] free quota image=%d/%d video=%d/%d",
		accountID, fq.ImageRemaining, fq.ImageTotal, fq.VideoRemaining, fq.VideoTotal)
}
