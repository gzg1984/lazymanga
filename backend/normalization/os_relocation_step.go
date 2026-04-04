package normalization

import (
	"errors"
	"fmt"
	"io"
	"lazyiso/models"
	"lazyiso/normalization/rulebook"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"gorm.io/gorm"
)

// OSRelocationStep relocates files by explicit RepoISO flags.
// - is_os=true: move into OS category (rule-based subdir when filename can be classified)
// - is_entertament=true: move into Entertainment
// - both false: do nothing
type OSRelocationStep struct{}

// OSRuleMatch is the exported match result for filename-based OS classification.
type OSRuleMatch struct {
	TypeName  string
	TargetDir string
	Keyword   string
}

var defaultOSRelocationRuleEngine = loadRuleEngineByBookSpec(0, osRuleBookName, osRuleBookVersion)

// GuessOSRuleByFileName tries to classify a file name using default OS relocation rules.
func GuessOSRuleByFileName(fileName string) (OSRuleMatch, bool) {
	result, err := defaultOSRelocationRuleEngine.Evaluate(rulebook.EvalInput{
		FileName:        fileName,
		IsOS:            false,
		IsEntertainment: false,
	})
	if err != nil || !result.Matched {
		return OSRuleMatch{}, false
	}

	return OSRuleMatch{
		TypeName:  result.RuleType,
		TargetDir: result.TargetDir,
		Keyword:   result.Keyword,
	}, true
}

func NewOSRelocationStep() RecordStep {
	return OSRelocationStep{}
}

func (s OSRelocationStep) Name() string {
	return "os-relocation"
}

func (s OSRelocationStep) Process(repoID uint, repoDB *gorm.DB, rootAbs string, record *models.RepoISO) error {
	autoNormalizeEnabled, err := repoAutoNormalizeEnabled(repoDB)
	if err != nil {
		return err
	}
	if !autoNormalizeEnabled {
		return nil
	}

	if record.IsOS && record.IsEntertament {
		return nil
	}

	fileName := strings.TrimSpace(record.FileName)
	if fileName == "" {
		fileName = filepath.Base(record.Path)
	}
	if fileName == "" {
		return nil
	}

	engine := getRuleEngineForRepo(repoID, repoDB)
	decision, err := engine.Evaluate(rulebook.EvalInput{
		FileName:        fileName,
		IsOS:            record.IsOS,
		IsEntertainment: record.IsEntertament,
	})
	if err != nil {
		return err
	}
	if !decision.Matched {
		return nil
	}

	sourceAbs, err := resolveRecordAbsPath(rootAbs, record.Path)
	if err != nil {
		return err
	}

	_, targetAbs, err := buildTargetPath(rootAbs, decision.TargetDir, fileName)
	if err != nil {
		return err
	}

	finalAbs, finalRelPath, moved, err := relocateFileWithUniqueTarget(sourceAbs, targetAbs, rootAbs)
	if err != nil {
		return err
	}
	_ = finalAbs

	updates := map[string]interface{}{}
	if decision.InferIsOS && !record.IsOS {
		updates["is_os"] = true
	}
	if !moved && record.Path == finalRelPath && filepath.Base(finalRelPath) == strings.TrimSpace(record.FileName) {
		if len(updates) == 0 {
			return nil
		}
		if err := repoDB.Model(&models.RepoISO{}).Where("id = ?", record.ID).Updates(updates).Error; err != nil {
			return err
		}
		record.IsOS = updates["is_os"] == true
		log.Printf("NormalizePipeline: inferred os type repo_id=%d id=%d type=%s keyword=%q rule_book=%s version=%s rule_id=%s path=%q", repoID, record.ID, decision.RuleType, decision.Keyword, decision.RuleBook, decision.RuleVersion, decision.RuleID, record.Path)
		return nil
	}

	updates["path"] = finalRelPath
	updates["file_name"] = filepath.Base(finalRelPath)

	if err := repoDB.Model(&models.RepoISO{}).Where("id = ?", record.ID).Updates(updates).Error; err != nil {
		return err
	}

	oldPath := record.Path
	record.Path = finalRelPath
	record.FileName = filepath.Base(finalRelPath)
	if decision.InferIsOS {
		record.IsOS = true
	}

	if moved {
		log.Printf("NormalizePipeline: relocated repo_id=%d id=%d type=%s keyword=%q rule_book=%s version=%s rule_id=%s from=%q to=%q", repoID, record.ID, decision.RuleType, decision.Keyword, decision.RuleBook, decision.RuleVersion, decision.RuleID, oldPath, finalRelPath)
	} else {
		log.Printf("NormalizePipeline: normalized path repo_id=%d id=%d type=%s keyword=%q rule_book=%s version=%s rule_id=%s path=%q", repoID, record.ID, decision.RuleType, decision.Keyword, decision.RuleBook, decision.RuleVersion, decision.RuleID, finalRelPath)
	}

	return nil
}

func repoAutoNormalizeEnabled(repoDB *gorm.DB) (bool, error) {
	var info models.RepoInfo
	err := repoDB.First(&info, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.AutoNormalize, nil
}

func buildTargetPath(rootAbs string, targetDir string, fileName string) (string, string, error) {
	cleanTargetDir := filepath.Clean(filepath.FromSlash(targetDir))
	if cleanTargetDir == "." {
		cleanTargetDir = ""
	}

	targetAbs := filepath.Join(rootAbs, cleanTargetDir, fileName)
	if !isPathWithinRoot(rootAbs, targetAbs) {
		return "", "", fmt.Errorf("target path out of root")
	}

	targetRel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return "", "", err
	}

	return filepath.ToSlash(targetRel), targetAbs, nil
}

func relocateFileWithUniqueTarget(sourceAbs string, targetAbs string, rootAbs string) (string, string, bool, error) {
	if sameFilePath(sourceAbs, targetAbs) {
		rel, err := filepath.Rel(rootAbs, targetAbs)
		if err != nil {
			return "", "", false, err
		}
		return targetAbs, filepath.ToSlash(rel), false, nil
	}

	if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
		return "", "", false, err
	}

	finalTargetAbs, err := findAvailableTargetPath(targetAbs)
	if err != nil {
		return "", "", false, err
	}

	if err := moveFileWithFallback(sourceAbs, finalTargetAbs); err != nil {
		return "", "", false, err
	}

	rel, err := filepath.Rel(rootAbs, finalTargetAbs)
	if err != nil {
		return "", "", true, err
	}
	return finalTargetAbs, filepath.ToSlash(rel), true, nil
}

func findAvailableTargetPath(targetAbs string) (string, error) {
	if _, err := os.Stat(targetAbs); err != nil {
		if os.IsNotExist(err) {
			return targetAbs, nil
		}
		return "", err
	}

	dir := filepath.Dir(targetAbs)
	fileName := filepath.Base(targetAbs)
	ext := filepath.Ext(fileName)
	base := strings.TrimSuffix(fileName, ext)

	for i := 1; i < 10000; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, i, ext))
		if _, err := os.Stat(candidate); err != nil {
			if os.IsNotExist(err) {
				return candidate, nil
			}
			return "", err
		}
	}

	return "", fmt.Errorf("unable to allocate unique target for %q", targetAbs)
}

func moveFileWithFallback(sourceAbs string, targetAbs string) error {
	if err := os.Rename(sourceAbs, targetAbs); err == nil {
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return err
	}

	if err := copyFileWithMode(sourceAbs, targetAbs); err != nil {
		return err
	}
	return os.Remove(sourceAbs)
}

func copyFileWithMode(sourceAbs string, targetAbs string) error {
	srcInfo, err := os.Stat(sourceAbs)
	if err != nil {
		return err
	}

	src, err := os.Open(sourceAbs)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(targetAbs, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode().Perm())
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return dst.Sync()
}

func sameFilePath(a string, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}
