package handlers

import (
	"fmt"
	"lazyiso/models"
	"lazyiso/normalization"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type legacyAddISORequest struct {
	Path string `json:"path"`
}

func openFlowLogf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("OPEN_FLOW: %s", msg)
	appendOpenFlowLog(msg)
}

func HandleOpen(c *gin.Context) {
	c.Set("open_flow", true)
	openFlowLogf("entry method=%s path=%s raw_query=%q remote=%s", c.Request.Method, c.Request.URL.Path, c.Request.URL.RawQuery, c.ClientIP())
	CreateISOs(c)
}

func isOpenFlow(c *gin.Context) bool {
	if v, ok := c.Get("open_flow"); ok {
		if flag, ok := v.(bool); ok && flag {
			return true
		}
	}
	fullPath := strings.TrimSpace(c.FullPath())
	return fullPath == "/open" || fullPath == "/api/open"
}

func parseAddISOPath(c *gin.Context) string {
	var req legacyAddISORequest
	if err := c.ShouldBindJSON(&req); err == nil {
		if v := strings.TrimSpace(req.Path); v != "" {
			return v
		}
	}

	if v := strings.TrimSpace(c.Query("file")); v != "" {
		return v
	}
	if v := strings.TrimSpace(c.Query("path")); v != "" {
		return v
	}
	return ""
}

func loadBasicRepository() (models.Repository, error) {
	var repo models.Repository
	err := db.Where("basic = ?", true).Order("id asc").First(&repo).Error
	if err == nil {
		return repo, nil
	}

	if err != nil && err != gorm.ErrRecordNotFound {
		return models.Repository{}, err
	}

	err = db.Where("name = ?", basicRepoName).Order("id asc").First(&repo).Error
	if err != nil {
		return models.Repository{}, err
	}
	return repo, nil
}

func CreateISOs(c *gin.Context) {
	openFlow := isOpenFlow(c)
	if openFlow {
		openFlowLogf("CreateISOs start full_path=%q url=%q", c.FullPath(), c.Request.URL.String())
	}

	inputPath := parseAddISOPath(c)
	if inputPath == "" {
		if openFlow {
			openFlowLogf("reject missing input path")
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
		return
	}
	if openFlow {
		openFlowLogf("parsed input path=%q", inputPath)
	}

	basicRepo, err := loadBasicRepository()
	if err != nil {
		if openFlow {
			openFlowLogf("load basic repository failed: %v", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query basic repository failed: " + err.Error()})
		return
	}
	if openFlow {
		openFlowLogf("basic repo resolved id=%d name=%q", basicRepo.ID, basicRepo.Name)
	}

	repoDB, rootAbs, _, err := openRepoScopedDB(basicRepo)
	if err != nil {
		if openFlow {
			openFlowLogf("open scoped repo db failed: %v", err)
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "prepare basic repo db failed: " + err.Error()})
		return
	}

	sourceRel, sourceAbs, err := resolveInternalSourcePath(inputPath)
	if err != nil {
		if openFlow {
			openFlowLogf("resolve internal source failed input=%q err=%v", inputPath, err)
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "path required and must be under internal root"})
		return
	}
	if openFlow {
		openFlowLogf("source resolved rel=%q abs=%q", sourceRel, sourceAbs)
	}

	srcInfo, err := os.Stat(sourceAbs)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "source file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "stat source file failed: " + err.Error()})
		return
	}
	if srcInfo.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source path is a directory"})
		return
	}
	if !strings.EqualFold(filepath.Ext(sourceAbs), ".iso") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only .iso files are supported"})
		return
	}

	targetAbs := sourceAbs
	if !isPathWithinRoot(rootAbs, sourceAbs) {
		targetAbs = filepath.Join(rootAbs, "manual_added", filepath.Base(sourceAbs))
		if !isPathWithinRoot(rootAbs, targetAbs) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "target path out of repo root"})
			return
		}

		if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "prepare target folder failed: " + err.Error()})
			return
		}

		finalTargetAbs, err := findRepoISOAvailableTargetPath(targetAbs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "allocate target path failed: " + err.Error()})
			return
		}

		if err := copyFile(sourceAbs, finalTargetAbs, srcInfo.Mode()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "copy source file failed: " + err.Error()})
			return
		}
		targetAbs = finalTargetAbs
	}

	relPath, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "resolve repo relative path failed: " + err.Error()})
		return
	}
	relPath = filepath.ToSlash(relPath)

	var exists models.RepoISO
	if err := repoDB.Where("path = ?", relPath).First(&exists).Error; err == nil {
		if openFlow {
			openFlowLogf("conflict existing iso id=%d rel=%q", exists.ID, relPath)
		}
		c.JSON(http.StatusConflict, gin.H{"error": "ISO文件已存在", "id": exists.ID})
		return
	} else if err != nil && err != gorm.ErrRecordNotFound {
		if openFlow {
			openFlowLogf("query existing repo iso failed rel=%q err=%v", relPath, err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query existing repo iso failed: " + err.Error()})
		return
	}

	targetInfo, err := os.Stat(targetAbs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "stat target file failed: " + err.Error()})
		return
	}

	row := models.RepoISO{
		FileName:  filepath.Base(targetAbs),
		Path:      relPath,
		SizeBytes: targetInfo.Size(),
		Tags:      models.ExtractTagsFromFileName(filepath.Base(targetAbs)),
	}
	if err := repoDB.Create(&row).Error; err != nil {
		if openFlow {
			openFlowLogf("create repo iso failed rel=%q err=%v", relPath, err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create repo iso failed: " + err.Error()})
		return
	}

	normalization.StartAsyncPostIndexNormalization(basicRepo.ID, repoDB, rootAbs, []models.RepoISO{row})
	if openFlow {
		openFlowLogf("success repo_id=%d iso_id=%d rel=%q copied=%t", basicRepo.ID, row.ID, row.Path, !sameRepoISOPath(sourceAbs, targetAbs))
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":        row.ID,
		"filename":  row.FileName,
		"path":      row.Path,
		"mountpath": row.MountPath,
		"md5":       row.MD5,
		"tags":      row.Tags,
		"ismounted": row.IsMounted,
		"repo_id":   basicRepo.ID,
		"source":    sourceRel,
		"copied":    !sameRepoISOPath(sourceAbs, targetAbs),
	})
}

/*
func UpdateTodo(c *gin.Context) {
	id := c.Param("id")
	var todo models.Todo

	if err := db.First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}

	var updatedTodo models.Todo
	if err := c.ShouldBindJSON(&updatedTodo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	todo.Todo = updatedTodo.Todo
	todo.IsCompleted = updatedTodo.IsCompleted

	if err := db.Save(&todo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, todo)
}

*/
