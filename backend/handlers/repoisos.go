package handlers

import (
	"errors"
	"lazyiso/models"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetRepoISOs reads current repository-local repoisos table.
func GetRepoISOs(c *gin.Context) {
	log.Printf("GetRepoISOs: start method=%s path=%s remote=%s", c.Request.Method, c.Request.URL.Path, c.ClientIP())
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}

	var repo models.Repository
	if err := db.First(&repo, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("GetRepoISOs: repo not found id=%s", id)
			c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
			return
		}
		log.Printf("GetRepoISOs: query failed id=%s error=%v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db query failed: " + err.Error()})
		return
	}

	repoDB, rootAbs, dbPath, err := openRepoScopedDB(repo)
	if err != nil {
		log.Printf("GetRepoISOs: open scoped db failed id=%s error=%v", id, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "prepare repo db failed: " + err.Error()})
		return
	}

	var rows []models.RepoISO
	if err := repoDB.Order("id desc").Find(&rows).Error; err != nil {
		log.Printf("GetRepoISOs: query repoisos failed id=%s db=%s error=%v", id, dbPath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query repoisos failed: " + err.Error()})
		return
	}

	log.Printf("GetRepoISOs: success id=%s root=%q db=%q total=%d", id, rootAbs, dbPath, len(rows))
	c.JSON(http.StatusOK, rows)
}
