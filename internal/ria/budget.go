package ria

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

const (
	HourLimit  = 30
	MonthLimit = 1000
)

// ErrBudget means the freemium average_price quota is gone; Match must stop, not mark lots scored.
var ErrBudget = errors.New("ria budget exhausted")

func HourBucket(t time.Time) string  { return t.UTC().Format("2006-01-02T15") }
func MonthBucket(t time.Time) string { return t.UTC().Format("2006-01") }

func ParseCount(stored, bucket string) int {
	k, n, ok := strings.Cut(stored, ":")
	if !ok || k != bucket {
		return 0
	}
	v, err := strconv.Atoi(n)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

func FormatCount(bucket string, n int) string {
	return bucket + ":" + strconv.Itoa(n)
}

func Allow(hourN, monthN int) bool {
	return hourN < HourLimit && monthN < MonthLimit
}
