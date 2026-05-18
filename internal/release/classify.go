package release

import (
	"path/filepath"
	"regexp"
	"strings"
)

type ReleaseShape string

const (
	ReleaseShapeVolumeOnly  ReleaseShape = "volume_only"
	ReleaseShapeChapterOnly ReleaseShape = "chapter_only"
	ReleaseShapeMixed       ReleaseShape = "mixed"
	ReleaseShapeUnknown     ReleaseShape = "unknown"
)

var (
	volumePattern  = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])v(?:ol(?:ume)?)?\s*0*\d+`)
	chapterPattern = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:ch|chapter)\s*0*\d+`)
	issuePattern   = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])0*\d{2,3}(?:\s*[-–]\s*0*\d{2,3})?(?:[^a-z0-9]|$)`)
)

func Classify(releaseTitle string, files []string) ReleaseShape {
	hasVolumes := false
	hasChapters := false

	for _, file := range files {
		parsed := ParseFile(file)
		if len(parsed.Content.Volumes) > 0 {
			hasVolumes = true
		}
		if len(parsed.Content.Chapters) > 0 {
			hasChapters = true
		}
	}

	if !hasVolumes && !hasChapters {
		title := strings.ToLower(releaseTitle)
		volumeSignal := volumePattern.MatchString(title)
		chapterSignal := chapterPattern.MatchString(title)

		for _, file := range files {
			base := strings.ToLower(filepath.Base(file))
			if volumePattern.MatchString(base) {
				volumeSignal = true
			}
			if chapterPattern.MatchString(base) {
				chapterSignal = true
			}
			if strings.Contains(base, "vol") || strings.Contains(base, "volume") {
				volumeSignal = true
			}
			if strings.Contains(base, "ch") || strings.Contains(base, "chapter") {
				chapterSignal = true
			}
		}

		if volumeSignal && chapterSignal {
			return ReleaseShapeMixed
		} else if volumeSignal {
			return ReleaseShapeVolumeOnly
		} else if chapterSignal {
			return ReleaseShapeChapterOnly
		} else if len(files) == 1 && issuePattern.MatchString(filepath.Base(files[0])) {
			return ReleaseShapeVolumeOnly
		} else {
			return ReleaseShapeUnknown
		}
	}

	switch {
	case hasVolumes && hasChapters:
		return ReleaseShapeMixed
	case hasVolumes:
		return ReleaseShapeVolumeOnly
	case hasChapters:
		return ReleaseShapeChapterOnly
	default:
		return ReleaseShapeUnknown
	}
}

func IsArchive(ext string) bool {
	switch ext {
	// technically rar should be here
	// but theyre just bad and usually not an actual proper release
	case ".cbz", ".cbr", ".zip":
		return true
	}
	return false
}
