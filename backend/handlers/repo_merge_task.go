package handlers

import (
	"fmt"
	"lazymanga/models"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	repoMergeTaskStatusRunning   = "running"
	repoMergeTaskStatusCompleted = "completed"
	repoMergeTaskStatusFailed    = "failed"
)

type RepoMergeTaskSnapshot struct {
	TaskID          string                  `json:"task_id"`
	Status          string                  `json:"status"`
	SourceRepoID    uint                    `json:"source_repo_id"`
	TargetRepoID    uint                    `json:"target_repo_id"`
	ProgressPercent float64                 `json:"progress_percent"`
	Processed       int                     `json:"processed"`
	Total           int                     `json:"total"`
	CurrentFile     string                  `json:"current_file"`
	CurrentStep     string                  `json:"current_step"`
	Message         string                  `json:"message"`
	Error           string                  `json:"error,omitempty"`
	StartedAt       time.Time               `json:"started_at"`
	UpdatedAt       time.Time               `json:"updated_at"`
	FinishedAt      *time.Time              `json:"finished_at,omitempty"`
	Flow            *RepoMergeFlowResult    `json:"flow,omitempty"`
	Cleanup         *RepoMergeCleanupResult `json:"cleanup,omitempty"`
}

type repoMergeTaskManager struct {
	mu    sync.RWMutex
	tasks map[string]*RepoMergeTaskSnapshot
	seq   uint64
}

var repoMergeTasks = newRepoMergeTaskManager()

func newRepoMergeTaskManager() *repoMergeTaskManager {
	return &repoMergeTaskManager{tasks: make(map[string]*RepoMergeTaskSnapshot)}
}

func (m *repoMergeTaskManager) createTask(sourceRepoID uint, targetRepoID uint) RepoMergeTaskSnapshot {
	now := time.Now()
	taskID := fmt.Sprintf("merge-%d-%06d", now.UnixNano(), atomic.AddUint64(&m.seq, 1))
	task := &RepoMergeTaskSnapshot{
		TaskID:          taskID,
		Status:          repoMergeTaskStatusRunning,
		SourceRepoID:    sourceRepoID,
		TargetRepoID:    targetRepoID,
		ProgressPercent: 0,
		Processed:       0,
		Total:           0,
		CurrentStep:     "start",
		Message:         "merge task started",
		StartedAt:       now,
		UpdatedAt:       now,
	}

	m.mu.Lock()
	m.tasks[taskID] = task
	m.mu.Unlock()

	return cloneRepoMergeTask(task)
}

func (m *repoMergeTaskManager) getTask(taskID string) (RepoMergeTaskSnapshot, bool) {
	m.mu.RLock()
	task, ok := m.tasks[taskID]
	m.mu.RUnlock()
	if !ok {
		return RepoMergeTaskSnapshot{}, false
	}
	return cloneRepoMergeTask(task), true
}

func (m *repoMergeTaskManager) updateProgress(taskID string, processed int, total int, currentPath string, currentStep string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return
	}
	if task.Status != repoMergeTaskStatusRunning {
		return
	}

	if processed >= 0 {
		task.Processed = processed
	}
	if total >= 0 {
		task.Total = total
	}
	if currentStep != "" {
		task.CurrentStep = currentStep
	}
	if strings.TrimSpace(currentPath) != "" {
		task.CurrentFile = filepath.Base(filepath.FromSlash(currentPath))
	}

	task.ProgressPercent = calculateRepoMergeProgressPercent(task.Processed, task.Total, task.Status)
	task.UpdatedAt = time.Now()
}

func (m *repoMergeTaskManager) setFlowResult(taskID string, flow RepoMergeFlowResult) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return
	}
	flowCopy := flow
	task.Flow = &flowCopy
	task.UpdatedAt = time.Now()
}

func (m *repoMergeTaskManager) setCleanupResult(taskID string, cleanup RepoMergeCleanupResult) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return
	}
	cleanupCopy := cleanup
	task.Cleanup = &cleanupCopy
	task.UpdatedAt = time.Now()
}

func (m *repoMergeTaskManager) markFailed(taskID string, errText string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return
	}
	now := time.Now()
	task.Status = repoMergeTaskStatusFailed
	task.Error = strings.TrimSpace(errText)
	task.Message = "merge task failed"
	task.FinishedAt = &now
	task.UpdatedAt = now
	task.ProgressPercent = calculateRepoMergeProgressPercent(task.Processed, task.Total, task.Status)
}

func (m *repoMergeTaskManager) markCompleted(taskID string, cleanup RepoMergeCleanupResult) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return
	}
	now := time.Now()
	task.Status = repoMergeTaskStatusCompleted
	task.Message = cleanup.Message
	if strings.TrimSpace(task.Message) == "" {
		task.Message = "merge task completed"
	}
	task.FinishedAt = &now
	task.UpdatedAt = now
	task.ProgressPercent = calculateRepoMergeProgressPercent(task.Processed, task.Total, task.Status)
}

func calculateRepoMergeProgressPercent(processed int, total int, status string) float64 {
	if total <= 0 {
		if status == repoMergeTaskStatusCompleted {
			return 100
		}
		return 0
	}
	if processed < 0 {
		processed = 0
	}
	if processed > total {
		processed = total
	}
	return float64(processed) * 100 / float64(total)
}

func cloneRepoMergeTask(task *RepoMergeTaskSnapshot) RepoMergeTaskSnapshot {
	cloned := *task
	if task.FinishedAt != nil {
		finished := *task.FinishedAt
		cloned.FinishedAt = &finished
	}
	if task.Flow != nil {
		flow := *task.Flow
		flow.Failures = append([]string(nil), task.Flow.Failures...)
		cloned.Flow = &flow
	}
	if task.Cleanup != nil {
		cleanup := *task.Cleanup
		cloned.Cleanup = &cleanup
	}
	return cloned
}

func runRepoMergeTask(taskID string, sourceRepo models.Repository, targetRepo models.Repository) {
	flowResult, err := ExecuteRepoMergeFlowWithProgress(sourceRepo, targetRepo, func(progress RepoMergeProgress) {
		repoMergeTasks.updateProgress(taskID, progress.Processed, progress.Total, progress.CurrentPath, progress.CurrentStep)
	})
	if err != nil {
		repoMergeTasks.markFailed(taskID, "execute merge flow failed: "+err.Error())
		return
	}
	repoMergeTasks.setFlowResult(taskID, flowResult)

	cleanupResult, err := ExecuteRepoMergeCleanup(sourceRepo)
	if err != nil {
		repoMergeTasks.markFailed(taskID, "execute cleanup failed: "+err.Error())
		return
	}
	repoMergeTasks.setCleanupResult(taskID, cleanupResult)
	repoMergeTasks.markCompleted(taskID, cleanupResult)
}

// GetRepoMergeTask returns merge task progress including percentage and current file.
func GetRepoMergeTask(c *gin.Context) {
	taskID := strings.TrimSpace(c.Param("taskId"))
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing taskId"})
		return
	}

	task, ok := repoMergeTasks.getTask(taskID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "merge task not found"})
		return
	}

	c.JSON(http.StatusOK, task)
}
