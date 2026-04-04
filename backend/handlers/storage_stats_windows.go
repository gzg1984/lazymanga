//go:build windows

package handlers

import "golang.org/x/sys/windows"

func getStorageStats(path string) (availableBytes int64, totalBytes int64, err error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}

	var freeBytesAvailable uint64
	var totalNumberOfBytes uint64
	var totalNumberOfFreeBytes uint64
	if err := windows.GetDiskFreeSpaceEx(pathPtr, &freeBytesAvailable, &totalNumberOfBytes, &totalNumberOfFreeBytes); err != nil {
		return 0, 0, err
	}

	return int64(freeBytesAvailable), int64(totalNumberOfBytes), nil
}
