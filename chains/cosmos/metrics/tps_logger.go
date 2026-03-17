package metrics

import (
	"fmt"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

const tpsLogInterval = 30 * time.Second

// TPSTracker tracks successful and attempted transaction counts for TPS logging.
type TPSTracker struct {
	successCount atomic.Int64
	attemptCount atomic.Int64
	done         chan struct{}
	logger       *zap.Logger
}

func NewTPSTracker(logger *zap.Logger) *TPSTracker {
	t := &TPSTracker{
		done:   make(chan struct{}),
		logger: logger,
	}
	go t.loop()
	return t
}

func (t *TPSTracker) Close() {
	close(t.done)
}

func (t *TPSTracker) RecordSend(succeeded, attempted int) {
	t.successCount.Add(int64(succeeded))
	t.attemptCount.Add(int64(attempted))
}

func (t *TPSTracker) loop() {
	ticker := time.NewTicker(tpsLogInterval)
	defer ticker.Stop()
	for {
		select {
		case <-t.done:
			return
		case <-ticker.C:
			succeeded := t.successCount.Swap(0)
			attempted := t.attemptCount.Swap(0)
			elapsed := tpsLogInterval.Seconds()
			t.logger.Info(fmt.Sprintf("last %s tps", tpsLogInterval.String()),
				zap.Float64("successful_tps", float64(succeeded)/elapsed),
				zap.Float64("attempted_tps", float64(attempted)/elapsed),
				zap.Int64("successful_txs", succeeded),
				zap.Int64("attempted_txs", attempted),
			)
		}
	}
}
