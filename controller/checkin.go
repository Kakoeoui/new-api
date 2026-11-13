package controller

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

// GetCheckinStatus 获取用户签到状态和历史记录
func GetCheckinStatus(c *gin.Context) {
	setting := operation_setting.GetCheckinSetting()
	if !setting.Enabled {
		common.ApiErrorT(c, "Check-in feature is not enabled")
		return
	}
	userId := c.GetInt("id")
	discordLinked, err := model.IsUserDiscordLinked(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 获取月份参数，默认为当前月份
	month := c.DefaultQuery("month", time.Now().Format("2006-01"))

	stats, err := model.GetUserCheckinStats(userId, month)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled":           setting.Enabled,
			"min_quota":         setting.MinQuota,
			"max_quota":         setting.MaxQuota,
			"discord_linked":    discordLinked,
			"require_discord":   true,
			"required_guild_id": model.CheckinDiscordGuildID,
			"minimum_role_id":   model.CheckinDiscordMinimumRoleID,
			"stats":             stats,
		},
	})
}

// DoCheckin 执行用户签到
func DoCheckin(c *gin.Context) {
	setting := operation_setting.GetCheckinSetting()
	if !setting.Enabled {
		common.ApiErrorT(c, "Check-in feature is not enabled")
		return
	}

	userId := c.GetInt("id")
	discordLinked, err := model.IsUserDiscordLinked(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !discordLinked {
		common.ApiErrorT(c, "You must link Discord before checking in.")
		return
	}

	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		common.ApiErrorT(c, "Sign in to check in")
		return
	}
	if _, _, err := service.ValidateLoginSession(identity); err != nil {
		writeSecurityOperationError(c, err)
		return
	}
	authorizationURL, err := createCheckinAuthorization(identity, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"authorization_url": authorizationURL})
}

// Try the existing Discord grant without another consent click; an interactive
// retry is allowed once if Discord requires sign-in or additional permission.
func createCheckinAuthorization(identity service.AuthIdentity, silent bool) (string, error) {
	discord := system_setting.GetDiscordSettings()
	if !discord.Enabled || discord.ClientId == "" || discord.ClientSecret == "" {
		return "", common.NewMessage("Discord verification is unavailable. Please try again later.")
	}
	payload := ""
	if silent {
		payload = "silent"
	}
	state, _, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose: model.AuthFlowPurposeOAuth, Provider: "discord", Intent: model.AuthFlowIntentCheckin,
		UserId: identity.UserID, SessionId: identity.SessionID, ExpiresAt: time.Now().Add(5 * time.Minute),
		Payload: payload,
	})
	if err != nil {
		return "", err
	}
	query := url.Values{
		"client_id": {discord.ClientId}, "redirect_uri": {fmt.Sprintf("%s/oauth/discord", system_setting.ServerAddress)},
		"response_type": {"code"}, "scope": {"identify guilds.members.read"}, "state": {state},
	}
	if silent {
		query.Set("prompt", "none")
	} else {
		query.Set("prompt", "consent")
	}
	return "https://discord.com/oauth2/authorize?" + query.Encode(), nil
}

// OAuth tokens stay in this request and are never written to the flow or session.
func handleOAuthCheckin(c *gin.Context, user *oauth.OAuthUser, token *oauth.OAuthToken, flow *model.AuthFlow) {
	checkin, err := model.UserCheckin(c.Request.Context(), flow.UserId, user.ProviderUserID, token.AccessToken)
	if err != nil {
		common.ApiErrorStatus(c, http.StatusOK, err, gin.H{"action": "checkin"})
		return
	}
	model.RecordLog(flow.UserId, model.LogTypeSystem, common.NewMessage("Daily check-in, received {{quota}}", map[string]any{"quota": logger.FormatQuota(checkin.QuotaAwarded)}))
	c.JSON(http.StatusOK, gin.H{"success": true, "action": "checkin", "data": gin.H{"quota_awarded": checkin.QuotaAwarded, "checkin_date": checkin.CheckinDate}})
}
