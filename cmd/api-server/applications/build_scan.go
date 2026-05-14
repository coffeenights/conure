package applications

import (
	"context"
	"log"
	"time"

	"github.com/coffeenights/conure/cmd/api-server/models"
)

// StartBuildAdoptionScan launches the periodic adoption ticker. Always-on
// per replica: every buildScanInterval the loop queries for remote builds
// whose lease is missing or expired and tries to acquire each. The lease's
// atomic conditional update ensures at most one replica wins any race; the
// loser quietly moves on.
//
// Returns immediately; the loop runs until ctx is canceled. Callers wire it
// into the server lifecycle in routes/groups.go.
func (a *ApiHandler) StartBuildAdoptionScan(ctx context.Context) {
	go func() {
		// First scan also doubles as startup recovery: pick up any
		// builds left in pending/building from a prior process. Run it
		// once immediately so a cold start doesn't wait an interval.
		a.scanAndAdopt(ctx)
		t := time.NewTicker(buildScanInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				a.scanAndAdopt(ctx)
			}
		}
	}()
}

// scanAndAdopt finds adoptable builds and attempts to claim each. Spawns a
// new watcher goroutine on every successful acquisition.
func (a *ApiHandler) scanAndAdopt(ctx context.Context) {
	builds, err := models.AdoptableBuilds(ctx, a.MongoDB, time.Now(), buildAdoptionBatch)
	if err != nil {
		log.Printf("build adoption scan: %v", err)
		return
	}
	for i := range builds {
		b := builds[i] // copy
		ok, err := b.TryAcquireLease(ctx, a.MongoDB, a.WatcherID, buildLeaseTTL)
		if err != nil {
			log.Printf("build adoption: lease acquire %s: %v", b.ID.Hex(), err)
			continue
		}
		if !ok {
			// Another replica beat us to it. Fine.
			continue
		}
		log.Printf("build adoption: adopted %s", b.ID.Hex())
		go a.watchBuildJob(context.Background(), b)
	}
}
