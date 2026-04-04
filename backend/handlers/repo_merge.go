package handlers

import (
	"errors"
	"lazymanga/models"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type repoMergeTransferRequest struct {
	SourceRepoID uint `json:"source_repo_id"`
	TargetRepoID uint `json:"target_repo_id"`
}

// RequestRepoMergeTransfer executes repo merge flow and then cleanup flow.
func RequestRepoMergeTransfer(c *gin.Context) {
	log.Printf("RequestRepoMergeTransfer: start method=%s path=%s remote=%s", c.Request.Method, c.Request.URL.Path, c.ClientIP())

	var req repoMergeTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("RequestRepoMergeTransfer: invalid request body error=%v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.SourceRepoID == 0 || req.TargetRepoID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_repo_id and target_repo_id are required"})
		return
	}
	if req.SourceRepoID == req.TargetRepoID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_repo_id and target_repo_id must be different"})
		return
	}

	var sourceRepo models.Repository
	if err := db.First(&sourceRepo, req.SourceRepoID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "source repo not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query source repo failed: " + err.Error()})
		return
	}

	var targetRepo models.Repository
	if err := db.First(&targetRepo, req.TargetRepoID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "target repo not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query target repo failed: " + err.Error()})
		return
	}

	task := repoMergeTasks.createTask(sourceRepo.ID, targetRepo.ID)
	go runRepoMergeTask(task.TaskID, sourceRepo, targetRepo)

	log.Printf("RequestRepoMergeTransfer: task started task_id=%s source=%d target=%d", task.TaskID, sourceRepo.ID, targetRepo.ID)
	c.JSON(http.StatusAccepted, gin.H{
		"message":        "merge transfer task started",
		"task_id":        task.TaskID,
		"status":         task.Status,
		"source_repo_id": sourceRepo.ID,
		"target_repo_id": targetRepo.ID,
		"progress":       task.ProgressPercent,
	})
}
