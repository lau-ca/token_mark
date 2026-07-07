package billingexpr

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	imageBillingTier1K = "1K"
	imageBillingTier2K = "2K"
	imageBillingTier4K = "4K"
)

func imageSizeTier(size any) string {
	tier, ok := classifyImageSizeTier(fmt.Sprint(size))
	if ok {
		return tier
	}
	return imageBillingTier2K
}

func classifyImageSizeTier(size string) (string, bool) {
	trimmed := strings.TrimSpace(size)
	normalized := strings.ToLower(trimmed)
	switch normalized {
	case "", "<nil>", "auto":
		return "", false
	case "1k":
		return imageBillingTier1K, true
	case "2k":
		return imageBillingTier2K, true
	case "4k":
		return imageBillingTier4K, true
	case "2048x2048", "2048x1152":
		return imageBillingTier2K, true
	case "3840x2160", "2160x3840":
		return imageBillingTier4K, true
	}

	width, height, ok := parseImageSizeDimensions(trimmed)
	if !ok {
		return "", false
	}
	maxEdge := width
	if height > maxEdge {
		maxEdge = height
	}
	switch {
	case maxEdge <= 1024:
		return imageBillingTier1K, true
	case maxEdge <= 2048:
		return imageBillingTier2K, true
	default:
		return imageBillingTier4K, true
	}
}

func parseImageSizeDimensions(size string) (int, int, bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(size)), "x")
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, false
	}
	height, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, false
	}
	if width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}
