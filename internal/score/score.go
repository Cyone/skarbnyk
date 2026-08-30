package score

import "strings"

func Discount(start, market float64) float64 {
	if market <= 0 || start < 0 {
		return 0
	}
	return 1 - start/market
}

func Family(sellingMethod string) string {
	s := strings.ToLower(sellingMethod)
	switch {
	case strings.Contains(s, "dutch"):
		return "dutch"
	case strings.Contains(s, "english"):
		return "english"
	default:
		return "other"
	}
}

func PassN(tenderAttempts int, previousAuctionID string) int {
	if tenderAttempts > 0 {
		return tenderAttempts
	}
	if previousAuctionID != "" {
		return 2
	}
	return 1
}
