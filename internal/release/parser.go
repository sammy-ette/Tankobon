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
	Special   bool
}

// trailingMeta matches a single trailing (...) or [...] group at the end of a string.
var trailingMeta = regexp.MustCompile(`\s*[\[\(][^\[\]\(\)]*[\]\)]\s*$`)
var bracketGroupRe = regexp.MustCompile(`\s*[\[\(][^\[\]\(\)]*[\]\)]\s*`)

var leadingGroupRe = regexp.MustCompile(`^\s*[\[\(]([^\]\)]+)[\]\)]\s*`)

var specialKeywordRe = regexp.MustCompile(`(?i)\bspecials?\b`)
var specialMarkerRe = regexp.MustCompile(`(?i)SP\d+`)
var mangaEditionRe = regexp.MustCompile(`(?i)\b(?:Omnibus(?:\s?Edition)?|Uncensored|(?:The\s+)?Complete\s+(?:Manga\s+)?(?:Collection|Series|Edition))\b`)
var multiSpaceRe = regexp.MustCompile(`\s{2,}`)
var yearGroupRe = regexp.MustCompile(`\s*[\[\(]\d{4}(?:-\d{4})?[\]\)]\s*`)
var pureNumberRe = regexp.MustCompile(`^\d+$`)
var volOrChapOnlyRe = regexp.MustCompile(`(?i)^(?:vol(?:ume)?|ch(?:apter)?)\s*\.?\s*\d*$`)

var duplicateVolumeRe = regexp.MustCompile(`(?i)(vol\.?|volume|v)(\s|_)*\d+.*?(vol\.?|volume|v)(\s|_)*\d+`)
var duplicateChapterRe = regexp.MustCompile(`(?i)(ch\.?|chapter|c)(\s|_)*\d+.*?(ch\.?|chapter|c)(\s|_)*\d+`)
var volumeRangeRe = regexp.MustCompile(`(?i)(vol\.?|v)(\s|_)?\d+(\.\d+)?-(vol\.?|v)(\s|_)?\d+(\.\d+)?`)
var chapterRangeRe = regexp.MustCompile(`(?i)(ch\.?|c)(\s|_)?\d+(\.\d+)?-(ch\.?|c)(\s|_)?\d+(\.\d+)?`)
var volumeNumberRe = regexp.MustCompile(`(?i)(vol\.?|volume|v)(\s|_)*(?P<Volume>\d+(\.\d+)?(-\d+(\.\d+)?)?)`)
var chapterNumberRe = regexp.MustCompile(`(?i)(ch\.?|chapter|c)(\s|_)*(?P<Chapter>\d+(\.\d+)?(-\d+(\.\d+)?)?)`)

var mangaSeriesRegex = []*regexp.Regexp{
	// Thai Volume: เล่ม n -> Volume n
	regexp.MustCompile(`(?i)(?P<Series>.+?)(เล่ม|เล่มที่)(\s)?(\.?)(\s|_)?(?P<Volume>\d+(\-\d+)?(\.\d+)?)`),
	// Russian Volume: Том n -> Volume n
	regexp.MustCompile(`(?i)(?P<Series>.+?)Том(а?)(\.?)(\s|_)?(?P<Volume>\d+(?:(\-)\d+)?)`),
	// Russian Volume: n Том -> Volume n
	regexp.MustCompile(`(?i)(?P<Series>.+?)(\s|_)?(?P<Volume>\d+(?:(\-)\d+)?)(\s|_)Том(а?)`),
	// Russian Chapter: n Главa -> Chapter n (lookahead/lookbehind dropped)
	regexp.MustCompile(`(?i)(?P<Series>.+?)\s\d+(\s|_)?(?P<Chapter>\d+(?:\.\d+|-\d+)?)(\s|_)(Глава|глава|Главы|Глава)`),
	// Russian Chapter: Главы n -> Chapter n
	regexp.MustCompile(`(?i)(?P<Series>.+?)(Глава|глава|Главы|Глава)(\.?)(\s|_)?(?P<Chapter>\d+(?:\.\d+|-\d+)?)`),
	// Grand Blue Dreaming - SP02
	regexp.MustCompile(`(?i)(?P<Series>.*)(\b|_|-|\s)(?:sp)\d`),
	// Mad Chimera World - Vol 005 - Chapter 026
	regexp.MustCompile(`(?i)(?P<Series>.+?)(\s|_|-)+(?:Vol(ume|\.)?(\s|_|-)+\d+)(\s|_|-)+(?:(?:Ch|Chapter)\.?)(\s|_|-)+(?P<Chapter>\d+)`),
	// Nagasarete Airantou - Vol. 30 Ch. 187.5 / Vol tbd
	regexp.MustCompile(`(?i)^(?P<Series>.+?)(?:\s*|_|\-\s*)+(?:Ch(?:apter|\.|)\s*\d+(?:\.\d+)?(?:\s*|_|\-\s*)+)?Vol(?:ume|\.|)\s*(?:\d+|tbd)(?:\s|_|\-\s*).+`),
	// Some Manga 001-050 as v01-05 (chapter range aliased to volume range)
	regexp.MustCompile(`(?i)(?P<Series>.+?)\s+\d+(?:\.\d+)?(?:-\d+(?:\.\d+)?)?\s+as\s+v\d`),
	// Ichiban_Ushiro_no_Daimaou_v04_ch34
	regexp.MustCompile(`(?i)(?P<Series>.*)(\b|_)v(?P<Volume>\d+-?\d*)(\s|_|-)`),
	// Gokukoku no Brynhildr - c001-008, Black Bullet - v4 c17
	regexp.MustCompile(`(?i)(?P<Series>.+?)( - )(?:v|vo|c|chapters|tome|t|ch)\d`),
	// Kedouin Makoto - Corpse Party Musume, Chapter 19
	regexp.MustCompile(`(?i)(?P<Series>.*)(?:, Chapter )(?P<Chapter>\d+)`),
	// Please Go Home, Akutsu-San! - Chapter 038.5 (lookahead dropped)
	regexp.MustCompile(`(?i)(?P<Series>.+?)(\s|_|-)(\s|_|-)((?:Chapter)|(?:Ch\.))(\s|_|-)(?P<Chapter>\d+)`),
	// One Piece - Digital Colored Comics Vol. 20
	regexp.MustCompile(`(?i)(?P<Series>.+?):? (\b|_|-)(vol|tome)\.?(\s|-|_)?\d+`),
	// Kyochuu Rettou Chapter 001 Volume 1
	regexp.MustCompile(`(?i)(?P<Series>.+?):?(\s|\b|_|-)Chapter(\s|\b|_|-)\d+(\s|\b|_|-)(vol)(ume)`),
	// Kyochuu Rettou T3, - Tome 3
	regexp.MustCompile(`(?i)(?P<Series>.+?):? (\b|_|-)(t\d+|tome(\b|_)\d+)`),
	// Kyochuu Rettou Volume 1
	regexp.MustCompile(`(?i)(?P<Series>.+?):? (\b|_|-)(vol)(ume)`),
	// Knights of Sidonia c000 (lookbehind dropped)
	regexp.MustCompile(`(?i)(?P<Series>.*?)\bc\d+\b`),
	// Tonikaku Cawaii [Volume 11], Darling in the FranXX - Volume 01
	regexp.MustCompile(`(?i)(?P<Series>.*)(?: _|-|\[|\()\s?(vol(ume)?|tome|t\d+)`),
	// Momo The Blood Taker - Chapter 027 / Grand Blue - SP02 (lookahead dropped)
	regexp.MustCompile(`(?i)^(?P<Series>.+?)(?:(ch(apter|\.)(\b|_|-|\s))|sp)\d`),
	// Historys Strongest Disciple Kenichi_v11_c90-98
	regexp.MustCompile(`(?i)(?P<Series>.*) (\b|_|-)(v|ch\.?|c|s)\d+`),
	// Hinowa ga CRUSH! 018 (2019) (Digital)
	regexp.MustCompile(`(?i)(?P<Series>.*)\s+(?P<Chapter>\d+)\s+(?:\(\d{4}\))\s`),
	// Goblin Slayer - Brand New Day 006.5 (2019) / Even the Elf Captain Wants to be a Maiden 001-050 (2023-2026)
	regexp.MustCompile(`(?i)(?P<Series>.*) (-)?(?P<Chapter>\d+(?:\.\d+|-\d+)?) \(\d{4}(?:-\d{4})?\)`),
	// Noblesse - Episode 429
	regexp.MustCompile(`(?i)(?P<Series>.*)(\s|_)(?:Episode|Ep\.?)(\s|_)(?P<Chapter>\d+(?:\.\d+|-\d+)?)`),
	// Akame ga KILL! ZERO (2016-2019)
	regexp.MustCompile(`(?i)(?P<Series>.*)\(\d`),
	// Tonikaku Kawaii (Ch 59-67)
	regexp.MustCompile(`(?i)(?P<Series>.*)(\s|_)\((c\s|ch\s|chapter\s)`),
	// Fullmetal Alchemist chapters 101-108
	regexp.MustCompile(`(?i)(?P<Series>.+?)(\s|_|\-)+?chapters(\s|_|\-)+?\d+(\s|_|\-)+?`),
	// It's Witching Time! 001 (Digital)
	regexp.MustCompile(`(?i)(?P<Series>.+?)(\s|_|\-)+?\d+(\s|_|\-)\(`),
	// Ichinensei_ni_Nacchattara_v01_ch01 (version dup check)
	regexp.MustCompile(`(?i)(?P<Series>.*)(v|s)\d+(-\d+)?(_|\s)`),
	// [Suihei Kiki]_Kasumi_Otoko_no_Ko_[Taruby]_v1.1
	regexp.MustCompile(`(?i)(?P<Series>.*)(v|s)\d+(-\d+)?`),
	// Black Bullet
	regexp.MustCompile(`(?i)(?P<Series>.*)(_)(v|vo|c|volume)( |_)\d+`),
	// [Hidoi]_Amaenaideyo_MS_vol01_chp02
	regexp.MustCompile(`(?i)(?P<Series>.*)( |_)(vol\d+)?( |_)(?:Chp\.? ?\d+)`),
	// Mahoutsukai to Deshi - Chp. 1
	regexp.MustCompile(`(?i)(?P<Series>.*)( |_)(?:Chp\.? ?\d+)`),
	// Corpse Party - Chapter 01 (lookahead dropped)
	regexp.MustCompile(`(?i)^(?P<Series>.*)( |_)Chapter( |_)(\d+)`),
	// Fullmetal Alchemist chapters (lookahead dropped)
	regexp.MustCompile(`(?i)^(?P<Series>.*)( |_)(chapters( |_)?)\d+-?\d*`),
	// Umineko episode/chapter (lookaheads + lookbehind dropped)
	regexp.MustCompile(`(?i)^(?P<Series>.*)( |_|-)(episode|chapter|(ch\.?) ?)\d+-?\d*`),
	// Baketeriya ch01-05 (lookahead dropped)
	regexp.MustCompile(`(?i)^(?P<Series>.*)ch\d+-?\d?`),
	// Magi - Ch.252-005
	regexp.MustCompile(`(?i)(?P<Series>.*)?( ?- ?)Ch\.\d+-?\d*`),
	// Korean: 권화話 (lookaheads dropped)
	regexp.MustCompile(`(?i)^(?P<Series>.+?)(-|_|\s|#)\d+(-\d+)?(권|화|話)`),
	// [BAA]_Darker_than_Black_Omake-1 (lookaheads dropped)
	regexp.MustCompile(`(?i)^(?P<Series>.+?)(-|_|\s|#)\d+(?:\.\d+)?(-\d+)?$`),
	// Akiiro Bousou Biyori - 01 (lookaheads + lookbehind dropped)
	regexp.MustCompile(`(?i)^(?P<Series>.+?)(\s|_|-)(ch|chapter)?\.?\d+(?:\.\d+)?-?\d*$`),
	// [BAA]_Darker_than_Black_c1 (lookahead dropped)
	regexp.MustCompile(`(?i)^(?P<Series>.*)( |_|-)(ch?)\d+`),
	// Japanese Volume: 第n巻
	regexp.MustCompile(`(?i)(?P<Series>.+?)第(?P<Volume>\d+(?:(\-)\d+)?)巻`),
}

var mangaVolumeRegex = []*regexp.Regexp{
	// Thai Volume: เล่ม n -> Volume n
	regexp.MustCompile(`(?i)(เล่ม|เล่มที่)(\s)?(\.?)(\s|_)?(?P<Volume>\d+(\-\d+)?(\.\d+)?)`),
	// Dance in the Vampire Bund v16-17, Tome 1
	regexp.MustCompile(`(?i)(?P<Series>.*)(\b|_)(v|tome(\s|_)?|t)(?P<Volume>\d+-?\d+)(\s|_)`),
	// Nagasarete Airantou - Vol. 30 Ch. 187.5 - Vol.31 Omake
	regexp.MustCompile(`(?i)^(?P<Series>.+?)(\s*Chapter\s*\d+)?(\s|_|\-\s)+((Vol(ume)?|tome)\.?(\s|_)?)(?P<Volume>\d+(\.\d+)?(\-\d+(\.\d+)?)?)(.+?|$)`),
	// Historys Strongest Disciple Kenichi_v11 (lookaheads dropped)
	regexp.MustCompile(`(?i)(?P<Series>.*)(\b|_)v(?P<Volume>\d+(\.\d)?(-\d+(\.\d)?)?)(\b|_)`),
	// Kodomo no Jikan vol. 10
	regexp.MustCompile(`(?i)(?P<Series>.*)(\b|_)(vol\.? ?)(?P<Volume>\d+(\.\d)?(-\d+)?(\.\d)?)`),
	// Killing Bites Vol. 0001
	regexp.MustCompile(`(?i)(vol\.? ?)(?P<Volume>\d+(\.\d)?)`),
	// Tonikaku Cawaii [Volume 11]
	regexp.MustCompile(`(?i)((volume|tome)\s)(?P<Volume>\d+(\.\d)?)`),
	// Tower Of God S01
	regexp.MustCompile(`(?i)(?P<Series>.*)(\b|_)((S|T)(?P<Volume>\d+)(\b|_))`),
	// vol_001 (MangaPy naming)
	regexp.MustCompile(`(?i)(vol_)(?P<Volume>\d+(\.\d)?)`),
	// Chinese: 第n卷/册
	regexp.MustCompile(`(?i)第(?P<Volume>\d+)(卷|册)`),
	// Chinese: 卷n/册n
	regexp.MustCompile(`(?i)(卷|册)(?P<Volume>\d+)`),
	// Korean: n권/장
	regexp.MustCompile(`(?i)제?(?P<Volume>\d+(\.\d+)?)(권|장)`),
	// Korean Season: 시즌n
	regexp.MustCompile(`(?i)시즌(?P<Volume>\d+(\-\d+)?)`),
	// Korean: n시즌
	regexp.MustCompile(`(?i)(?P<Volume>\d+(\-|~)?\d+?)시즌`),
	// Korean Season range: 시즌n-m
	regexp.MustCompile(`(?i)시즌(?P<Volume>\d+(\-|~)?\d+?)`),
	// Japanese: n巻
	regexp.MustCompile(`(?i)(?P<Volume>\d+(?:(\-)\d+)?)巻`),
	// Russian: Том n
	regexp.MustCompile(`(?i)Том(а?)(\.?)(\s|_)?(?P<Volume>\d+(?:(\-)\d+)?)`),
	// Russian: n Том
	regexp.MustCompile(`(?i)(\s|_)?(?P<Volume>\d+(?:(\-)\d+)?)(\s|_)Том(а?)`),
}

var mangaChapterRegex = []*regexp.Regexp{
	// Thai Chapter: บทที่/ตอนที่ n
	regexp.MustCompile(`(?i)(?P<Volume>((เล่ม|เล่มที่))?(\s|_)?\.?\d+)(\s|_)(บทที่|ตอนที่)\.?(\s|_)?(?P<Chapter>\d+)`),
	// Historys Strongest Disciple c90-98, c90.5-100.5
	regexp.MustCompile(`(?i)(\b|_)(c|ch)(\.?\s?)(?P<Chapter>(\d+(\.\d)?)(-c?\d+(\.\d)?)?)\b`),
	// [Suihei Kiki]_Kasumi_Otoko_no_Ko_[Taruby]_v1.1
	regexp.MustCompile(`(?i)v\d+\.(\s|_)(?P<Chapter>\d+(?:\.\d+|-\d+)?)`),
	// Umineko - Episode 3 #02
	regexp.MustCompile(`(?i)^(?P<Series>.*)(?: |_)#(?P<Chapter>\d+)`),
	// Green Worldz - Chapter 027 (lookbehind dropped)
	regexp.MustCompile(`(?i)^(?P<Series>.*)\sChapter\s(?P<Chapter>\d+(?:\.?[\d-]+)?)`),
	// Russian: Главы n
	regexp.MustCompile(`(?i)(Глава|глава|Главы|Глава)(\.?)(\s|_)?(?P<Chapter>\d+(?:\.\d+|-\d+)?)`),
	// Chinese: 第n话
	regexp.MustCompile(`(?i)第(?P<Chapter>\d+)话`),
	// Korean: 제n화/회/장
	regexp.MustCompile(`(?i)제?(?P<Chapter>\d+\.?\d+)(회|화|장)`),
	// Japanese/Korean: 第n話
	regexp.MustCompile(`(?i)第?(?P<Chapter>\d+(?:\.\d+|-\d+)?)話`),
	// Russian: n Главa (lookaheads/lookbehind dropped)
	regexp.MustCompile(`(?i)\s\d+(\s|_)?(?P<Chapter>\d+(?:\.\d+|-\d+)?)(\s|_)(Глава|глава|Главы|Глава)`),
	// Fullmetal Alchemist chapters 101-108
	regexp.MustCompile(`(?i)^(?P<Series>.+?)\schapter(?:s)?\s(?P<Chapter>\d+-\d+)`),
	// Hinowa ga CRUSH! 018 (2019) (lookbehinds + lookahead dropped)
	regexp.MustCompile(`(?i)^(?P<Series>.+?)\s(\d\s)?(?P<Chapter>\d+(?:\.\d+|-\d+)?)(?:\s\(\d{4}\))?(\b|_|-)`),
	// Tower Of God S01 014
	regexp.MustCompile(`(?i)(?P<Series>.*)\sS(?P<Volume>\d+)\s(?P<Chapter>\d+(?:\.\d+|-\d+)?)`),
	// Beelzebub_01_[Noodles], Beelzebub_153b_RHS (complex lookahead simplified)
	regexp.MustCompile(`(?i)^(?P<Series>.+?)(\s|_)(?P<Chapter>\.?\d+(?:\.\d+|-\d+)?)(?P<Part>b)?(\s|_|\[|\()`),
	// Yumekui-Merry_DKThias_Chapter21
	regexp.MustCompile(`(?i)Chapter(?P<Chapter>\d+(-\d+)?)`),
	// [Hidoi]_Amaenaideyo_MS_vol01_chp02
	regexp.MustCompile(`(?i)(?P<Series>.*)(\s|_)(vol\d+)?(\s|_)Chp\.? ?(?P<Chapter>\d+)`),
	// Vol 1 Chapter 2
	regexp.MustCompile(`(?i)(?P<Volume>((vol|volume|v))?(\s|_)?\.?\d+)(\s|_)(Chp|Chapter)\.?(\s|_)?(?P<Chapter>\d+)`),
}

func stripAllBrackets(s string) string {
	for {
		next := strings.TrimSpace(multiSpaceRe.ReplaceAllString(bracketGroupRe.ReplaceAllString(s, " "), " "))
		if next == s {
			return s
		}
		s = next
	}
}

func cleanTitle(s string) string {
	s = mangaEditionRe.ReplaceAllString(s, "")
	s = stripAllBrackets(s)
	s = strings.Trim(s, "\x00\t\r -,")
	s = multiSpaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func matchedGroup(re *regexp.Regexp, s, name string) string {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	idx := re.SubexpIndex(name)
	if idx < 0 || idx >= len(m) {
		return ""
	}
	return m[idx]
}

func parseMangaSeries(base string) string {
	for _, re := range mangaSeriesRegex {
		if v := matchedGroup(re, base, "Series"); v != "" {
			return cleanTitle(v)
		}
	}
	return ""
}

func parseMangaVolume(base string) string {
	base = removeDuplicateVolume(base)
	for _, re := range mangaVolumeRegex {
		if v := matchedGroup(re, base, "Volume"); v != "" {
			return formatVolChapValue(v)
		}
	}
	return ""
}

func parseMangaChapter(base string) string {
	base = removeDuplicateChapter(base)
	for _, re := range mangaChapterRegex {
		m := re.FindStringSubmatch(base)
		if m == nil {
			continue
		}
		chIdx := re.SubexpIndex("Chapter")
		if chIdx < 0 || chIdx >= len(m) || m[chIdx] == "" {
			continue
		}
		val := m[chIdx]
		partIdx := re.SubexpIndex("Part")
		if partIdx >= 0 && partIdx < len(m) && m[partIdx] != "" && !strings.Contains(val, ".") {
			val = val + ".5"
		}
		return formatVolChapValue(val)
	}
	return ""
}

func formatVolChapValue(value string) string {
	if !strings.Contains(value, "-") {
		return removeLeadingZeroes(value)
	}
	tokens := strings.SplitN(value, "-", 2)
	from := removeLeadingZeroes(tokens[0])
	to := tokens[1]
	if len(to) > 0 && (to[0] == 'c' || to[0] == 'C') {
		to = to[1:]
	}
	return from + "-" + removeLeadingZeroes(to)
}

func removeLeadingZeroes(s string) string {
	s = strings.TrimLeft(s, "0")
	if s == "" || s == "." {
		return "0"
	}
	return s
}

func removeDuplicateVolume(s string) string {
	if volumeRangeRe.MatchString(s) {
		return s
	}
	m := duplicateVolumeRe.FindStringSubmatchIndex(s)
	if m == nil {
		return s
	}
	firstStart := m[2]
	vm := volumeNumberRe.FindStringSubmatchIndex(s[firstStart:])
	if vm == nil {
		return s
	}
	firstVolEnd := firstStart + vm[1]
	vm2 := volumeNumberRe.FindStringSubmatchIndex(s[firstVolEnd:])
	if vm2 == nil {
		return s
	}
	secondStart := firstVolEnd + vm2[0]
	return strings.TrimRight(s[:secondStart], " -_")
}

func removeDuplicateChapter(s string) string {
	if chapterRangeRe.MatchString(s) {
		return s
	}
	m := duplicateChapterRe.FindStringSubmatchIndex(s)
	if m == nil {
		return s
	}
	firstStart := m[2]
	cm := chapterNumberRe.FindStringSubmatchIndex(s[firstStart:])
	if cm == nil {
		return s
	}
	firstChEnd := firstStart + cm[1]
	cm2 := chapterNumberRe.FindStringSubmatchIndex(s[firstChEnd:])
	if cm2 == nil {
		return s
	}
	secondStart := firstChEnd + cm2[0]
	return strings.TrimRight(s[:secondStart], " -_")
}

func addToContent(m map[string]struct{}, val string) {
	if val == "" {
		return
	}
	if idx := strings.Index(val, "-"); idx > 0 {
		start, end := val[:idx], val[idx+1:]
		for _, v := range expandRange(start, end) {
			m[v] = struct{}{}
		}
	} else {
		m[NormalizeContentNumber(val)] = struct{}{}
	}
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
	}

	base = strings.TrimSpace(multiSpaceRe.ReplaceAllString(yearGroupRe.ReplaceAllString(base, " "), " "))

	seriesBase := stripAllBrackets(base)
	result.Series = parseMangaSeries(seriesBase)
	if result.Series == "" {
		result.Series = cleanTitle(base)
	}

	// If the leading bracket group was actually the series title (not a scanlation group),
	// the remainder yields a non-title like "Volume 01" or a bare number.
	if isBadSeries(result.Series) && result.Group != "" {
		result.Series = result.Group
		result.Group = ""
	}

	addToContent(result.Content.Volumes, parseMangaVolume(base))
	addToContent(result.Content.Chapters, parseMangaChapter(base))

	splitDualTitles(&result)
	result.Special = specialMarkerRe.MatchString(base) ||
		isDecimalChapterContent(result.Content) ||
		hasSpecialKeyword(base)
	return result
}

func isBadSeries(s string) bool {
	return s == "" || pureNumberRe.MatchString(s) || volOrChapOnlyRe.MatchString(s)
}

func hasSpecialKeyword(s string) bool {
	return specialKeywordRe.MatchString(s)
}

func isDecimalChapterContent(c repository.MangaContent) bool {
	if len(c.Chapters) == 0 {
		return false
	}
	for ch := range c.Chapters {
		if strings.Contains(ch, ".") {
			return true
		}
	}
	return false
}

// splitDualTitles handles release names with two titles separated by " | ".
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

// stripTrailingMeta removes trailing (...) and [...] groups from s, repeating until none remain.
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
