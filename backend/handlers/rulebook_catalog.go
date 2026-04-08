package handlers

import (
	"bytes"
	"encoding/json"
	"lazymanga/normalization"
	"lazymanga/normalization/rulebook"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type createRuleBookPayload struct {
	Name      string          `json:"name"`
	Version   string          `json:"version"`
	Content   json.RawMessage `json:"content"`
	Overwrite bool            `json:"overwrite"`
}

type updateRuleBookPayload struct {
	Content json.RawMessage `json:"content"`
}

func defaultRuleBookTemplate(name string, version string) rulebook.RuleBook {
	return rulebook.RuleBook{
		Name:    strings.TrimSpace(strings.ToLower(name)),
		Version: strings.TrimSpace(strings.ToLower(version)),
		Scan: rulebook.ScanSpec{
			Extensions: []string{".cbz", ".zip", ".rar", ".7z", ".pdf"},
			DirectoryRules: []rulebook.DirectoryScanRule{{
				Name:         "image-folder",
				Extensions:   []string{".jpg", ".jpeg", ".png", ".webp"},
				MinFileCount: 5,
			}},
		},
		Rules: []rulebook.Rule{},
	}
}

// ListRuleBooks returns currently discoverable rulebook files and validation status.
func ListRuleBooks(c *gin.Context) {
	books := normalization.ListAvailableRuleBooks()
	catalog := normalization.GetRuleBookCatalogInfo()
	validCount := 0
	for _, b := range books {
		if b.Valid {
			validCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total":        len(books),
		"valid_count":  validCount,
		"items":        books,
		"writable_dir": catalog.WritableDir,
		"builtin_dir":  catalog.BuiltinDir,
		"search_dirs":  catalog.SearchDirs,
	})
}

// CreateRuleBook creates a new user rulebook JSON inside the writable user-data rulebooks directory.
func CreateRuleBook(c *gin.Context) {
	var req createRuleBookPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	var book rulebook.RuleBook
	trimmedContent := strings.TrimSpace(string(req.Content))
	if trimmedContent != "" && trimmedContent != "null" {
		decoder := json.NewDecoder(bytes.NewReader(req.Content))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&book); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rulebook content: " + err.Error()})
			return
		}
	} else {
		book = defaultRuleBookTemplate(req.Name, req.Version)
	}

	if strings.TrimSpace(req.Name) != "" {
		book.Name = req.Name
	}
	if strings.TrimSpace(req.Version) != "" {
		book.Version = req.Version
	}

	saved, err := normalization.SaveUserRuleBook(book, req.Overwrite)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "already exists") {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{
			"error":        err.Error(),
			"writable_dir": normalization.GetRuleBookCatalogInfo().WritableDir,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "rulebook created",
		"item":         saved,
		"writable_dir": normalization.GetRuleBookCatalogInfo().WritableDir,
	})
}

func normalizeRuleBookRequestQuery(c *gin.Context) (string, string, error) {
	name := strings.TrimSpace(c.Query("name"))
	version := strings.TrimSpace(c.Query("version"))
	return normalizeBindingInput(name, version)
}

// GetRuleBookContent returns the raw JSON and metadata for the selected rulebook.
func GetRuleBookContent(c *gin.Context) {
	name, version, err := normalizeRuleBookRequestQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	item, raw, book, err := normalization.GetRuleBookFileContent(name, version)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"item":    item,
		"book":    book,
		"content": string(raw),
	})
}

// UpdateRuleBookContent updates an editable user rulebook in place.
func UpdateRuleBookContent(c *gin.Context) {
	name, version, err := normalizeRuleBookRequestQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	currentItem, _, _, err := normalization.GetRuleBookFileContent(name, version)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	if !currentItem.Editable {
		c.JSON(http.StatusForbidden, gin.H{"error": "built-in rulebook is read-only; please create a new user rulebook instead"})
		return
	}

	var req updateRuleBookPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	trimmedContent := strings.TrimSpace(string(req.Content))
	if trimmedContent == "" || trimmedContent == "null" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content required"})
		return
	}

	decoder := json.NewDecoder(bytes.NewReader(req.Content))
	decoder.DisallowUnknownFields()
	var book rulebook.RuleBook
	if err := decoder.Decode(&book); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rulebook content: " + err.Error()})
		return
	}
	book.Name = name
	book.Version = version

	saved, err := normalization.SaveUserRuleBook(book, true)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "rulebook updated",
		"item":    saved,
	})
}
