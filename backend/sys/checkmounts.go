package sys

import (
	"path/filepath"
)

func GetFullPathFromDBSubPath(subPath string) string {
	root := "/lzcapp/run/mnt/home"
	return filepath.Join(root, subPath)
}
