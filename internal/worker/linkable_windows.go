//go:build windows

package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func checkLinkable(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("stat source: %w", err)
	}
	if _, err := os.Stat(dst); err != nil {
		return fmt.Errorf("stat library path: %w", err)
	}
	if !strings.EqualFold(filepath.VolumeName(src), filepath.VolumeName(dst)) {
		return fmt.Errorf("source and library are on different volumes")
	}
	return nil
}
