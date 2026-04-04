package handlers

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type openFlowLogEntry struct {
	Time    string `json:"time"`
	Message string `json:"message"`
}

const openFlowLogLimit = 300

var (
	openFlowLogMu   sync.Mutex
	openFlowLogList []openFlowLogEntry
)

func appendOpenFlowLog(message string) {
	openFlowLogMu.Lock()
	defer openFlowLogMu.Unlock()

	openFlowLogList = append(openFlowLogList, openFlowLogEntry{
		Time:    time.Now().UTC().Format(time.RFC3339Nano),
		Message: message,
	})
	if len(openFlowLogList) > openFlowLogLimit {
		overflow := len(openFlowLogList) - openFlowLogLimit
		openFlowLogList = append([]openFlowLogEntry(nil), openFlowLogList[overflow:]...)
	}
}

func GetOpenFlowLogs(c *gin.Context) {
	openFlowLogMu.Lock()
	items := make([]openFlowLogEntry, len(openFlowLogList))
	copy(items, openFlowLogList)
	openFlowLogMu.Unlock()

	c.JSON(200, gin.H{
		"count": len(items),
		"items": items,
	})
}
