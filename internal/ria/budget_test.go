package ria

import (
	"testing"
	"time"
)

func TestParseCount(t *testing.T) {
	if ParseCount("", "2026-08-30T20") != 0 {
		t.Fatal("empty")
	}
	if ParseCount("2026-08-30T19:29", "2026-08-30T20") != 0 {
		t.Fatal("new hour resets")
	}
	if ParseCount("2026-08-30T20:7", "2026-08-30T20") != 7 {
		t.Fatal("same hour")
	}
	if ParseCount("garbage", "2026-08-30T20") != 0 {
		t.Fatal("garbage")
	}
}

func TestAllow(t *testing.T) {
	if !Allow(0, 0) || !Allow(HourLimit-1, MonthLimit-1) {
		t.Fatal("under")
	}
	if Allow(HourLimit, 0) || Allow(0, MonthLimit) {
		t.Fatal("at limit")
	}
}

func TestBucketsUTC(t *testing.T) {
	ts := time.Date(2026, 8, 30, 20, 15, 0, 0, time.UTC)
	if HourBucket(ts) != "2026-08-30T20" {
		t.Fatal(HourBucket(ts))
	}
	if MonthBucket(ts) != "2026-08" {
		t.Fatal(MonthBucket(ts))
	}
	if FormatCount("2026-08-30T20", 3) != "2026-08-30T20:3" {
		t.Fatal("format")
	}
}
