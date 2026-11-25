package analysis

import (
	"testing"

	"strikeye/internal/mapcontrol"
)

func TestSparklineScalesAndTrims(t *testing.T) {
	if got := sparkline(nil); got != "" {
		t.Fatalf("empty = %q", got)
	}
	got := sparkline([]float64{-1, 0, 1})
	if got != "▁▅█" {
		t.Fatalf("got %q", got)
	}
	long := make([]float64, 400)
	for i := range long {
		long[i] = 1
	}
	if n := len([]rune(sparkline(long))); n != 90 {
		t.Fatalf("width = %d", n)
	}
}

func TestControlBarDirection(t *testing.T) {
	if got := controlBar(1.0); got != "██████████████" {
		t.Fatalf("ct bar = %q", got)
	}
	if got := controlBar(-0.5); got != "═══════███████" {
		t.Fatalf("t bar = %q", got)
	}
}

func TestControlBucketsGroupBySecond(t *testing.T) {
	res := mapcontrol.Result{Frames: []mapcontrol.Frame{
		{Round: 1, TimeSec: 0.0, Net: 0},
		{Round: 1, TimeSec: 0.4, Net: 1},
		{Round: 2, TimeSec: 5.0, Net: -1},
	}}
	buckets, order := controlBuckets(res)
	if len(order) != 2 || order[0] != 0 || order[1] != 5 {
		t.Fatalf("order = %v", order)
	}
	if len(buckets[0]) != 2 || buckets[0][1] != 1 {
		t.Fatalf("bucket 0 = %v", buckets[0])
	}
}

func TestControlCalloutsRanksByDeviation(t *testing.T) {
	res := mapcontrol.Result{Callouts: []mapcontrol.CalloutControl{
		{Callout: "Neutral", AvgNet: 0.01},
		{Callout: "StrongCT", AvgNet: 0.9},
		{Callout: "StrongT", AvgNet: -0.8},
	}}
	got := controlCallouts(res, 2)
	if len(got) != 2 || got[0].Callout != "StrongCT" || got[1].Callout != "StrongT" {
		t.Fatalf("got %+v", got)
	}
}
