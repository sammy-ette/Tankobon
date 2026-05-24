package release

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"tankobon/internal/repository"
)

type ParsedFile struct {
	Series    string
	AltSeries []string
	Group     string
	Content   repository.MangaContent
}

var rangePattern = regexp.MustCompile(`(?i)(?:vol(?:ume)?|v)\.?\s*(\d+(?:\.\d+)?)\s*[-–]\s*(?:vol(?:ume)?|v)?\.?\s*(\d+(?:\.\d+)?)`)

var asVolPattern = regexp.MustCompile(`(?i)(?:c(?:h(?:apter)?)?\.?\s*)?(\d+(?:\.\d+)?)\s*[-–]\s*(\d+(?:\.\d+)?)\s+as\s+(?:v(?:ol(?:ume)?)?\.?\s*)(\d+(?:\.\d+)?)\s*[-–]\s*(?:v(?:ol(?:ume)?)?\.?\s*)?(\d+(?:\.\d+)?)`)

// trailingMeta matches a single trailing (...) or [...] group at the end of a string.
var trailingMeta = regexp.MustCompile(`\s*[\[\(][^\[\]\(\)]*[\]\)]\s*$`)

var leadingGroupRe = regexp.MustCompile(`^\s*[\[\(]([^\]\)]+)[\]\)]\s*`)
var trailingGroupRe = regexp.MustCompile(`[\[\(]([^\[\]\(\)]+)[\]\)]\s*$`)
var yearRe = regexp.MustCompile(`^\d{4}(-\d{4})?$`)

// chapterIndicatorRe detects a chapter marker (c/ch/chapter + digit) in the
// text before a bracket range, so we can tell "[v06-12]" is context metadata.
var chapterIndicatorRe = regexp.MustCompile(`(?i)\bc(?:h(?:apter)?)?\.?\s*\d`)

var nonGroupWords = map[string]bool{
	"digital": true, "print": true, "web": true, "raw": true,
	"scan": true, "official": true, "omnibus": true, "lq": true, "hq": true,
}

func looksLikeGroup(s string) bool {
	if yearRe.MatchString(s) {
		return false
	}
	return !nonGroupWords[strings.ToLower(s)]
}

type pattern struct {
	regex      *regexp.Regexp
	volumeIdx  int
	chapterIdx int
}

var patterns = []pattern{
	{regex: regexp.MustCompile(`(?i)^(.+?)\s+(?:vol|v)\s*\.?\s*(\d+(?:\.\d+)?)\s+(?:ch|chapter)\s*\.?\s*(\d+(?:\.\d+)?)`), volumeIdx: 2, chapterIdx: 3},
	{regex: regexp.MustCompile(`(?i)^(.+?)[_\s]v\s*(\d+(?:\.\d+)?)[_\s]c\s*(\d+(?:\.\d+)?)`), volumeIdx: 2, chapterIdx: 3},
	{regex: regexp.MustCompile(`(?i)^(.+?)\s+(?:vol|v)\s*\.?\s*(\d+(?:\.\d+)?)`), volumeIdx: 2, chapterIdx: -1},
	{regex: regexp.MustCompile(`(?i)^(.+?)\s*-\s*(?:vol|v)\s*\.?\s*(\d+(?:\.\d+)?)`), volumeIdx: 2, chapterIdx: -1},
	{regex: regexp.MustCompile(`(?i)^(.+?)\s+(?:ch|chapter)\s*\.?\s*(\d+(?:\.\d+)?)`), volumeIdx: -1, chapterIdx: 2},
	{regex: regexp.MustCompile(`(?i)^(.+?)[_\s]v\s*(\d+(?:\.\d+)?)`), volumeIdx: 2, chapterIdx: -1},
	{regex: regexp.MustCompile(`(?i)^(.+?)[_\s]c\s*(\d+(?:\.\d+)?)`), volumeIdx: -1, chapterIdx: 2},
	{regex: regexp.MustCompile(`(?i)(?:vol|v)\s*\.?\s*(\d+(?:\.\d+)?)`), volumeIdx: 1, chapterIdx: -1},
	{regex: regexp.MustCompile(`(?i)(?:ch|chapter)\s*\.?\s*(\d+(?:\.\d+)?)`), volumeIdx: -1, chapterIdx: 1},
}

func ParseFile(filename string) ParsedFile {
	base := filepath.Base(filename)
	base = strings.TrimSuffix(base, filepath.Ext(base))

	base = strings.ReplaceAll(base, "_", " ")
	base = strings.TrimSpace(base)

	result := ParsedFile{Content: repository.NewContent()}

	if m := leadingGroupRe.FindStringSubmatch(base); len(m) == 2 {
		result.Group = strings.TrimSpace(m[1])
		base = leadingGroupRe.ReplaceAllString(base, "")
	} else if m := trailingGroupRe.FindStringSubmatch(base); len(m) == 2 {
		candidate := strings.TrimSpace(m[1])
		if looksLikeGroup(candidate) {
			result.Group = candidate
		}
	}

	if m := asVolPattern.FindStringSubmatch(base); len(m) == 5 {
		for _, v := range expandRange(m[1], m[2]) {
			result.Content.Chapters[v] = struct{}{}
		}
		for _, v := range expandRange(m[3], m[4]) {
			result.Content.Volumes[v] = struct{}{}
		}
		loc := asVolPattern.FindStringIndex(base)
		if loc != nil && loc[0] > 0 {
			s := strings.TrimSpace(base[:loc[0]])
			s = strings.TrimSuffix(s, "-")
			s = strings.TrimSpace(s)
			if s != "" && !isNumeric(s) {
				result.Series = s
			}
		}
		if result.Series == "" {
			result.Series = strings.TrimSpace(base)
		}
		splitDualTitles(&result)
		return result
	}

	if loc := rangePattern.FindStringIndex(base); loc != nil {
		// If the range sits inside brackets (e.g. [v06-12]) and the text before it
		// contains a chapter indicator, the bracket is context metadata, not content.
		inBracket := loc[0] > 0 && (base[loc[0]-1] == '[' || base[loc[0]-1] == '(')
		if !inBracket || !chapterIndicatorRe.MatchString(base[:loc[0]]) {
			m := rangePattern.FindStringSubmatch(base)
			for _, v := range expandRange(m[1], m[2]) {
				result.Content.Volumes[v] = struct{}{}
			}
			if loc[0] > 0 {
				s := strings.TrimSpace(base[:loc[0]])
				s = strings.TrimSuffix(s, "-")
				s = strings.TrimRight(s, "[(")
				s = strings.TrimSpace(s)
				if s != "" && !isNumeric(s) {
					result.Series = s
				}
			}
			if result.Series == "" {
				result.Series = strings.TrimSpace(base)
			}
			splitDualTitles(&result)
			return result
		}
	}

	for _, p := range patterns {
		matches := p.regex.FindStringSubmatch(base)
		if len(matches) < 2 {
			continue
		}

		if len(matches) > 1 && matches[1] != "" {
			s := strings.TrimSpace(matches[1])
			s = strings.TrimSuffix(s, "-")
			s = strings.TrimSpace(s)
			if result.Series == "" && !isNumeric(s) {
				result.Series = s
			}
		}

		if p.volumeIdx > 0 && p.volumeIdx < len(matches) && matches[p.volumeIdx] != "" {
			result.Content.Volumes[NormalizeContentNumber(matches[p.volumeIdx])] = struct{}{}
		}

		if p.chapterIdx > 0 && p.chapterIdx < len(matches) && matches[p.chapterIdx] != "" {
			result.Content.Chapters[NormalizeContentNumber(matches[p.chapterIdx])] = struct{}{}
		}

		if result.Series != "" || len(result.Content.Volumes) > 0 || len(result.Content.Chapters) > 0 {
			break
		}
	}

	if result.Series == "" {
		result.Series = strings.TrimSpace(base)
	}

	splitDualTitles(&result)
	return result
}

// splitDualTitles handles release names with two titles separated by " | ",
// e.g. "Takane no Ran-san | Ran the Peerless Beauty (2019-2021) (Digital) (1r0n)".
// It splits the series name, strips trailing metadata from each part, and
// stores the first as Series and the rest in AltSeries.
func splitDualTitles(result *ParsedFile) {
	if !strings.Contains(result.Series, " | ") {
		return
	}
	parts := strings.Split(result.Series, " | ")
	result.Series = stripTrailingMeta(strings.TrimSpace(parts[0]))
	for _, part := range parts[1:] {
		if clean := stripTrailingMeta(strings.TrimSpace(part)); clean != "" {
			result.AltSeries = append(result.AltSeries, clean)
		}
	}
}

// stripTrailingMeta removes trailing (...) and [...] groups from s, repeating
// until none remain. Used to clean metadata like year, format, and group tags.
func stripTrailingMeta(s string) string {
	for {
		next := strings.TrimSpace(trailingMeta.ReplaceAllString(s, ""))
		if next == s {
			return s
		}
		s = next
	}
}

func NormalizeContentNumber(n string) string {
	n = strings.TrimSpace(n)
	if n == "" {
		return n
	}

	parts := strings.SplitN(n, ".", 2)
	whole := strings.TrimLeft(parts[0], "0")
	if whole == "" {
		whole = "0"
	}
	if len(parts) == 1 {
		return whole
	}
	return whole + "." + parts[1]
}

func expandRange(start, end string) []string {
	s, errS := strconv.ParseFloat(start, 64)
	e, errE := strconv.ParseFloat(end, 64)
	if errS != nil || errE != nil {
		return []string{NormalizeContentNumber(start), NormalizeContentNumber(end)}
	}
	if s > e {
		s, e = e, s
	}
	si, ei := int64(s), int64(e)
	if s != float64(si) || e != float64(ei) {
		return []string{NormalizeContentNumber(start), NormalizeContentNumber(end)}
	}
	out := make([]string, 0, ei-si+1)
	for i := si; i <= ei; i++ {
		out = append(out, strconv.FormatInt(i, 10))
	}
	return out
}

func isNumeric(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && c != '.' && c != '-' && c != ' ' {
			return false
		}
	}
	return true
}
