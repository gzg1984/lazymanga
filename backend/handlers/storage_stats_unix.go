//go:build !windows

package handlers

import "syscall"

func getStorageStats(path string) (availableBytes int64, totalBytes int64, err error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, err
	}

	availableBytes = int64(stat.Bavail) * int64(stat.Bsize)
	totalBytes = int64(stat.Blocks) * int64(stat.Bsize)
	return availableBytes, totalBytes, nil
}
