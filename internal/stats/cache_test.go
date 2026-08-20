package stats_test

import (
	"testing"

	"github.com/convin/webhook-ingest/internal/stats"
)

func TestCacheRecordAccumulates(t *testing.T) {
	c := stats.NewCache()

	c.Record("acc_1", 30)
	c.Record("acc_1", 12)
	c.Record("acc_2", 5)

	got, ok := c.Get("acc_1")
	if !ok || got.CallCount != 2 || got.TotalDurationSec != 42 {
		t.Fatalf("acc_1: got %+v ok=%v, want CallCount=2 TotalDurationSec=42 ok=true", got, ok)
	}

	other, ok := c.Get("acc_2")
	if !ok || other.CallCount != 1 || other.TotalDurationSec != 5 {
		t.Fatalf("acc_2: got %+v ok=%v, want CallCount=1 TotalDurationSec=5 ok=true", other, ok)
	}
}

func TestCacheGetUnknownAccountIsZero(t *testing.T) {
	c := stats.NewCache()
	got, ok := c.Get("nobody")
	if ok {
		t.Fatalf("got ok=true for unknown account, want false")
	}
	if got.CallCount != 0 || got.TotalDurationSec != 0 {
		t.Fatalf("got %+v, want zero value", got)
	}
}