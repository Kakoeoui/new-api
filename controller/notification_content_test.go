package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestNotificationContentPermissionsAndExistingReaders(t *testing.T) {
	setupAuthFlowControllerTest(t)
	previousOptions := common.OptionMap
	previousConsole := *console_setting.GetConsoleSetting()
	common.OptionMap = map[string]string{}
	t.Cleanup(func() { common.OptionMap = previousOptions; *console_setting.GetConsoleSetting() = previousConsole })
	require.NoError(t, model.DB.AutoMigrate(&model.Option{}))
	engine := gin.New()
	engine.GET("/api/notification-content", middleware.AdminAuth(), GetNotificationContent)
	engine.PUT("/api/notification-content", middleware.AdminAuth(), UpdateNotificationContent)
	engine.GET("/api/notice", GetNotice)
	payload := `{"notice":"Maintenance notice","announcements":[{"id":42,"content":"Launch","extra":"Details","publishDate":"2026-10-20T12:00:00Z","type":"success"}],"announcements_enabled":true}`
	for _, role := range []int{0, common.RoleCommonUser, common.RoleAdminUser, common.RoleRootUser} {
		t.Run(fmt.Sprint(role), func(t *testing.T) {
			token := ""
			if role != 0 {
				user := model.User{Username: fmt.Sprintf("notification-editor-%d", role), Role: role, Status: common.UserStatusEnabled, AuthVersion: 1, Group: "default", AffCode: fmt.Sprintf("notification-%d", role)}
				require.NoError(t, model.DB.Create(&user).Error)
				session := model.UserSession{SID: fmt.Sprintf("notification-session-%d", role), UserID: user.Id, Version: 1, UserAuthVersion: 1, Status: model.UserSessionStatusActive, RefreshHash: fmt.Sprintf("hash-%d", role), ExpiresAt: time.Now().Unix() + 3600}
				require.NoError(t, model.CreateUserSession(&session))
				var err error
				token, _, err = service.IssueAccessToken(service.AuthIdentity{UserID: user.Id, SessionID: session.SID, UserAuthVersion: 1, SessionVersion: 1})
				require.NoError(t, err)
			}
			for _, method := range []string{http.MethodGet, http.MethodPut} {
				request := httptest.NewRequest(method, "/api/notification-content", strings.NewReader(payload))
				request.Header.Set("Content-Type", "application/json")
				if token != "" {
					request.Header.Set("Authorization", "Bearer "+token)
				}
				response := httptest.NewRecorder()
				engine.ServeHTTP(response, request)
				if role == 0 {
					require.Equal(t, http.StatusUnauthorized, response.Code)
					continue
				}
				if role < common.RoleAdminUser {
					require.Equal(t, http.StatusForbidden, response.Code)
					continue
				}
				require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			}
			if role < common.RoleAdminUser {
				return
			}
			read := httptest.NewRecorder()
			engine.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/api/notice", nil))
			require.Contains(t, read.Body.String(), "Maintenance notice")
			require.Equal(t, "Launch", console_setting.GetAnnouncements()[0]["content"])
			require.True(t, console_setting.GetConsoleSetting().AnnouncementsEnabled)
			var option model.Option
			require.NoError(t, model.DB.First(&option, "key = ?", "Notice").Error)
			require.Equal(t, "Maintenance notice", option.Value)
			request := httptest.NewRequest(http.MethodPut, "/api/notification-content", strings.NewReader(`{"notice":"bad overwrite","announcements":[{"content":"invalid date","publishDate":"bad"}],"announcements_enabled":true}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+token)
			invalid := httptest.NewRecorder()
			engine.ServeHTTP(invalid, request)
			require.Equal(t, http.StatusBadRequest, invalid.Code)
			require.Equal(t, "Maintenance notice", common.OptionMap["Notice"])
		})
	}
}
