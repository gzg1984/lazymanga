package handlers

import (
	"lazyiso/sys"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// DownloadHandler 通过 path 查询参数提供文件下载
// GET /download?path=ISO%2Fubuntu.iso
func DownloadHandler(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		log.Printf("DownloadHandler: missing path query param")
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing path"})
		return
	}

	realPath := sys.GetFullPathFromDBSubPath(path)
	log.Printf("Download requested, dbSubPath=%s mappedTo=%s", path, realPath)

	// 检查文件是否存在且不是目录
	fi, err := os.Stat(realPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("DownloadHandler: file does not exist: %s", realPath)
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found", "path": realPath})
			return
		}
		log.Printf("DownloadHandler: stat error for %s: %v", realPath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if fi.IsDir() {
		log.Printf("DownloadHandler: path is a directory, not a file: %s", realPath)
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is a directory", "path": realPath})
		return
	}

	filename := filepath.Base(realPath)
	// 使用 c.FileAttachment 更好地处理 headers
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.File(realPath)
}
