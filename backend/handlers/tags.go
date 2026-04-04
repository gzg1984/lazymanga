package handlers

import (
	"lazymanga/models"
	"strings"

	"github.com/gin-gonic/gin"
)

func QueryAllTags(c *gin.Context) {
	var isos []models.ISOs
	if err := db.Find(&isos).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	tagSet := make(map[string]struct{})
	for _, iso := range isos {
		tags := strings.Split(iso.Tags, ",")
		for _, tag := range tags {
			t := strings.TrimSpace(tag)
			if t != "" {
				tagSet[t] = struct{}{}
			}
		}
	}
	var tagArr []string
	for t := range tagSet {
		tagArr = append(tagArr, t)
	}
	c.JSON(200, tagArr)
}
