package handlers

import (
	"encoding/json"
	"errors"
	"lazyiso/models"
	"lazyiso/normalization"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	ruleBookNamePattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	ruleBookVersionPattern = regexp.MustCompile(`^v[0-9]+$`)
)

type repoRuleBookBindingPayload struct {
	RuleBookName    string `json:"rulebook_name"`
	RuleBookVersion string `json:"rulebook_version"`
}

func normalizeBindingInput(name string, version string) (string, string, error) {
	n := strings.TrimSpace(strings.ToLower(name))
	v := strings.TrimSpace(strings.ToLower(version))
	if n == "" {
		n = "noop"
	}
	if v == "" {
		v = "v1"
	}

	if !ruleBookNamePattern.MatchString(n) {
		return "", "", errors.New("invalid rulebook_name")
	}
	if !ruleBookVersionPattern.MatchString(v) {
		return "", "", errors.New("invalid rulebook_version")
	}

	return n, v, nil
}

func readRepoInfoByID(c *gin.Context) (models.Repository, *gorm.DB, models.RepoInfo, bool) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return models.Repository{}, nil, models.RepoInfo{}, false
	}

	var repo models.Repository
	if err := db.First(&repo, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
			return models.Repository{}, nil, models.RepoInfo{}, false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db query failed: " + err.Error()})
		return models.Repository{}, nil, models.RepoInfo{}, false
	}

	repoDB, _, _, err := openRepoScopedDB(repo)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prepare repo db failed: " + err.Error()})
		return models.Repository{}, nil, models.RepoInfo{}, false
	}

	info, err := EnsureRepoInfoFromRepository(repoDB, repo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "load repo_info failed: " + err.Error()})
		return models.Repository{}, nil, models.RepoInfo{}, false
	}

	return repo, repoDB, info, true
}

// GetRepoRuleBookBinding returns repo-level rulebook binding from repo_info.flags_json.
func GetRepoRuleBookBinding(c *gin.Context) {
	repo, _, info, ok := readRepoInfoByID(c)
	if !ok {
		return
	}

	resolved := normalization.ResolveEffectiveRuleBookBinding(info)

	c.JSON(http.StatusOK, gin.H{
		"repo_id":            repo.ID,
		"rulebook_name":      resolved.Name,
		"rulebook_version":   resolved.Version,
		"binding_source":     resolved.Source,
		"explicit_binding":   resolved.Explicit,
		"resolution_note":    resolved.ResolutionNote,
		"raw_flags_json":     info.FlagsJSON,
	})
}

// UpdateRepoRuleBookBinding updates repo-level rulebook binding in repo_info.flags_json.
func UpdateRepoRuleBookBinding(c *gin.Context) {
	repo, repoDB, info, ok := readRepoInfoByID(c)
	if !ok {
		return
	}

	var req repoRuleBookBindingPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	name, version, err := normalizeBindingInput(req.RuleBookName, req.RuleBookVersion)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	path, book, err := normalization.ValidateRuleBookSpec(name, version)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":            "rulebook spec invalid or file unreadable",
			"rulebook_name":    name,
			"rulebook_version": version,
			"path":             path,
			"details":          err.Error(),
		})
		return
	}

	flags := map[string]interface{}{}
	if strings.TrimSpace(info.FlagsJSON) != "" {
		if err := json.Unmarshal([]byte(info.FlagsJSON), &flags); err != nil {
			flags = map[string]interface{}{}
		}
	}
	flags["rulebook_name"] = name
	flags["rulebook_version"] = version

	nextFlags, err := json.Marshal(flags)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encode flags_json failed: " + err.Error()})
		return
	}

	if err := repoDB.Model(&models.RepoInfo{}).Where("id = ?", info.ID).Update("flags_json", string(nextFlags)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update repo_info failed: " + err.Error()})
		return
	}

	normalization.InvalidateRuleBookEngineCache()

	c.JSON(http.StatusOK, gin.H{
		"message":          "repo rulebook binding updated",
		"repo_id":          repo.ID,
		"rulebook_name":    name,
		"rulebook_version": version,
		"path":             path,
		"book_name":        book.Name,
		"rule_count":       len(book.Rules),
		"flags_json":       string(nextFlags),
	})
}
