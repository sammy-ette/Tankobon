package clients

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/mmcdole/gofeed"
)

// matches v.3 or v.1-3
var muVolumeRe = regexp.MustCompile(`\bv\.(\d+)(?:-(\d+))?`)

// matches c.23, c.15.1, or c.15.1-15.3 / c.13-15
var muChapterRe = regexp.MustCompile(`\bc\.(\d+)(?:\.\d+)?(?:-(\d+))?`)

func maxInt(a, b int) int {
	if b > a {
		return b
	}
	return a
}

func parseRangeMax(m []string) int {
	lo, _ := strconv.Atoi(m[1])
	if m[2] == "" {
		return lo
	}
	hi, _ := strconv.Atoi(m[2])
	return maxInt(lo, hi)
}

func FetchLatestCounts(ctx context.Context, mangaUpdatesID uint) (totalVolumes, totalChapters int, err error) {
	url := fmt.Sprintf("https://api.mangaupdates.com/v1/series/%d/rss", mangaUpdatesID)

	fp := gofeed.NewParser()
	feed, err := fp.ParseURLWithContext(url, ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("mangaupdates rss %d: %w", mangaUpdatesID, err)
	}

	for _, item := range feed.Items {
		if m := muVolumeRe.FindStringSubmatch(item.Title); len(m) == 3 {
			if n := parseRangeMax(m); n > totalVolumes {
				totalVolumes = n
			}
		}
		if m := muChapterRe.FindStringSubmatch(item.Title); len(m) == 3 {
			if n := parseRangeMax(m); n > totalChapters {
				totalChapters = n
			}
		}
	}

	return totalVolumes, totalChapters, nil
}
