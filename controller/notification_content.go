package controller

import (
	"net/http"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/gin-gonic/gin"
)

type notificationContent struct {
	Notice        string           `json:"notice"`
	Announcements []map[string]any `json:"announcements"`
	Enabled       bool             `json:"announcements_enabled"`
}

func GetNotificationContent(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	content := notificationContent{
		Notice:        common.OptionMap["Notice"],
		Announcements: console_setting.GetAnnouncements(),
		Enabled:       console_setting.GetConsoleSetting().AnnouncementsEnabled,
	}
	common.OptionMapRWMutex.RUnlock()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": content})
}

func UpdateNotificationContent(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	var content notificationContent
	if c.ShouldBindJSON(&content) != nil || content.Announcements == nil || utf8.RuneCountInString(content.Notice) > 50000 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid notification content"})
		return
	}
	raw, err := common.Marshal(content.Announcements)
	if err != nil || console_setting.ValidateConsoleSettings(string(raw), "Announcements") != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid notification content"})
		return
	}
	enabled := "false"
	if content.Enabled {
		enabled = "true"
	}
	err = model.UpdateOptionsBulk(map[string]string{
		"Notice":                                content.Notice,
		"console_setting.announcements":         string(raw),
		"console_setting.announcements_enabled": enabled,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Unable to save notifications"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": content})
}
