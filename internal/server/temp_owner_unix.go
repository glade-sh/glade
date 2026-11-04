//go:build darwin || linux

package server

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func trustedLightningTempParent(path string) bool {
	for _, part := range strings.Split(path, string(filepath.Separator)) {
		if part == ".." {
			return false
		}
	}
	return trustedLightningPathAncestry(path)
}

// Walk every symlink target, including intermediate paths, before trusting it.
func trustedLightningPathAncestry(path string) bool {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	parts := strings.Split(absolute, string(filepath.Separator))
	current := string(filepath.Separator)
	root, err := os.Lstat(current)
	if err != nil || !root.IsDir() || (root.Mode().Perm()&0022 != 0 && root.Mode()&os.ModeSticky == 0) {
		return false
	}
	rootStat, ok := root.Sys().(*syscall.Stat_t)
	if !ok || (rootStat.Uid != 0 && rootStat.Uid != uint32(os.Geteuid())) {
		return false
	}
	hops := 0
	for len(parts) > 0 {
		part := parts[0]
		parts = parts[1:]
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			current = filepath.Dir(current)
			continue
		}
		candidate := filepath.Join(current, part)
		info, err := os.Lstat(candidate)
		if err != nil {
			return false
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || (stat.Uid != 0 && stat.Uid != uint32(os.Geteuid())) {
			return false
		}
		if info.Mode()&os.ModeSymlink != 0 {
			hops++
			if hops > 40 {
				return false
			}
			target, err := os.Readlink(candidate)
			if err != nil || target == "" {
				return false
			}
			if filepath.IsAbs(target) {
				current = string(filepath.Separator)
			}
			parts = append(strings.Split(target, string(filepath.Separator)), parts...)
			continue
		}
		if !info.IsDir() || (info.Mode().Perm()&0022 != 0 && info.Mode()&os.ModeSticky == 0) {
			return false
		}
		current = candidate
	}
	return true
}
