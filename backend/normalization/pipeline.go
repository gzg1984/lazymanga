package normalization

import (
	"fmt"
	"lazymanga/models"
	"log"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
)

// RecordStep defines a normalize step executed after records are inserted.
type RecordStep interface {
	Name() string
	Process(repoID uint, repoDB *gorm.DB, rootAbs string, record *models.RepoISO) error
}

// Pipeline runs normalize steps for each indexed record.
type Pipeline struct {
	steps            []RecordStep
	progressInterval int
}

func newDefaultPipeline() *Pipeline {
	return &Pipeline{
		steps: []RecordStep{
			NewOSRelocationStep(),
			NewFileSizeBackfillStep(),
			NewMD5BackfillStep(),
		},
		progressInterval: 20,
	}
}

// DefaultStepNames returns currently enabled normalize step names.
func DefaultStepNames() []string {
	steps := newDefaultPipeline().steps
	names := make([]string, 0, len(steps))
	for _, step := range steps {
		names = append(names, step.Name())
	}
	return names
}

// StartAsyncPostIndexNormalization starts async normalization for inserted records.
func StartAsyncPostIndexNormalization(repoID uint, repoDB *gorm.DB, rootAbs string, records []models.RepoISO) {
	newDefaultPipeline().startAsync(repoID, repoDB, rootAbs, records)
}

// StartAsyncMetadataBackfill backfills metadata fields only (size + md5) for existing records.
func StartAsyncMetadataBackfill(repoID uint, repoDB *gorm.DB, rootAbs string, records []models.RepoISO) {
	p := &Pipeline{
		steps: []RecordStep{
			NewFileSizeBackfillStep(),
			NewMD5BackfillStep(),
		},
		progressInterval: 20,
	}
	p.startAsync(repoID, repoDB, rootAbs, records)
}

func (p *Pipeline) startAsync(repoID uint, repoDB *gorm.DB, rootAbs string, records []models.RepoISO) {
	if len(records) == 0 {
		log.Printf("NormalizePipeline: skipped repo_id=%d reason=no records", repoID)
		return
	}

	recordsCopy := make([]models.RepoISO, len(records))
	copy(recordsCopy, records)

	stepNames := make([]string, 0, len(p.steps))
	for _, step := range p.steps {
		stepNames = append(stepNames, step.Name())
	}

	go func() {
		startedAt := time.Now()
		total := len(recordsCopy)
		processed := 0
		recordFailureCount := 0
		stepFailureCount := make(map[string]int)

		log.Printf("NormalizePipeline: start repo_id=%d total=%d steps=%v", repoID, total, stepNames)

		for i := range recordsCopy {
			record := &recordsCopy[i]
			recordFailed := false
			for _, step := range p.steps {
				if err := step.Process(repoID, repoDB, rootAbs, record); err != nil {
					recordFailed = true
					stepFailureCount[step.Name()]++
					log.Printf("NormalizePipeline: step failed repo_id=%d step=%s id=%d path=%q error=%v", repoID, step.Name(), record.ID, record.Path, err)
				}
			}

			if recordFailed {
				recordFailureCount++
			}

			processed = i + 1
			if processed%p.progressInterval == 0 || processed == total {
				log.Printf("NormalizePipeline: progress repo_id=%d done=%d/%d record_failed=%d elapsed=%s", repoID, processed, total, recordFailureCount, time.Since(startedAt).Truncate(time.Millisecond))
			}
		}

		log.Printf("NormalizePipeline: done repo_id=%d total=%d record_failed=%d step_failures=%v elapsed=%s", repoID, total, recordFailureCount, stepFailureCount, time.Since(startedAt).Truncate(time.Millisecond))
	}()
}

func resolveRecordAbsPath(rootAbs string, relativePath string) (string, error) {
	absPath := filepath.Join(rootAbs, filepath.FromSlash(relativePath))
	if !isPathWithinRoot(rootAbs, absPath) {
		return "", fmt.Errorf("path out of root")
	}
	return absPath, nil
}

func isPathWithinRoot(root string, target string) bool {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && rel != "")
}
