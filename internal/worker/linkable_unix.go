//go:build !windows

package worker

import (
	"fmt"
	"os"
	"syscall"
)

func checkLinkable(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}
	dstInfo, err := os.Stat(dst)
	if err != nil {
		return fmt.Errorf("stat library path: %w", err)
	}
	srcDev := srcInfo.Sys().(*syscall.Stat_t).Dev
	dstDev := dstInfo.Sys().(*syscall.Stat_t).Dev
	if srcDev != dstDev {
		return fmt.Errorf("source and library are on different filesystems; hardlinking is not possible — move them to the same volume")
	}
	return nil
}
