package controller

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRecoveryTest(t *testing.T) (*gorm.DB, model.User, model.Token) {
	t.Helper()
	db, dialect := openTaskDialectDatabase(t, &model.User{}, &model.Token{}, &model.TwoFA{}, &model.TwoFABackupCode{}, &model.PasskeyCredential{}, &model.UserSession{}, &model.AuthFlow{}, &model.AuditLog{}, &model.Log{})
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldMain, oldLog := common.MainDatabaseType(), common.LogDatabaseType()
	oldRedis, oldMemory, oldPassword, oldSecret := common.RedisEnabled, common.MemoryCacheEnabled, common.PasswordLoginEnabled, common.SessionSecret
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(dialect, dialect)
	common.RedisEnabled, common.MemoryCacheEnabled, common.PasswordLoginEnabled = false, false, true
	common.SessionSecret = "recovery-test-session-secret"
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.SetDatabaseTypes(oldMain, oldLog)
		common.RedisEnabled, common.MemoryCacheEnabled, common.PasswordLoginEnabled, common.SessionSecret = oldRedis, oldMemory, oldPassword, oldSecret
	})
	user := model.User{Username: "recover-target", Status: common.UserStatusEnabled, Role: common.RoleCommonUser, Group: "default", AuthVersion: 1, AffCode: "recovery-target"}
	require.NoError(t, db.Create(&user).Error)
	key := model.Token{UserId: user.Id, Key: "recovery-test-api-key", Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true}
	require.NoError(t, db.Create(&key).Error)
	return db, user, key
}

func TestRecoveryRejectsInactiveCredentials(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		expires int64
	}{
		{"disabled", common.TokenStatusDisabled, -1},
		{"expired", common.TokenStatusEnabled, time.Now().Add(-time.Hour).Unix()},
		{"expired status", common.TokenStatusExpired, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, user, key := setupRecoveryTest(t)
			require.NoError(t, db.Model(&key).Updates(map[string]any{"status": tc.status, "expired_time": tc.expires}).Error)
			response := securityEnrollmentRequest(http.MethodPost, "/api/user/recover-by-key", fmt.Sprintf(`{"username":%q,"api_key":%q}`, user.Username, "sk-"+key.Key), "", service.AuthIdentity{}, RecoverAccountByApiKey)
			assert.Contains(t, response.Body.String(), `"success":false`)
			assert.NotContains(t, response.Body.String(), "access_token")
			assert.Empty(t, response.Result().Cookies())
			var count int64
			require.NoError(t, db.Model(&model.UserSession{}).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}

func TestRecoveryRequiresSecondFactorBeforeIssuingSession(t *testing.T) {
	db, user, key := setupRecoveryTest(t)
	const secret = "JBSWY3DPEHPK3PXP"
	require.NoError(t, db.Create(&model.TwoFA{UserId: user.Id, Secret: secret, IsEnabled: true}).Error)
	source := model.User{Username: "temporary-discord", DiscordId: "123456789012345678", Quota: 12345, Status: common.UserStatusEnabled, Role: common.RoleCommonUser, Group: "default", AuthVersion: 1, AffCode: "source"}
	require.NoError(t, db.Create(&source).Error)
	bundle, err := service.CreateLoginSession(source.Id, "oauth:discord", "127.0.0.1", "recovery-test")
	require.NoError(t, err)
	identity, err := service.ParseAccessToken(bundle.AccessToken)
	require.NoError(t, err)
	response := securityEnrollmentRequest(http.MethodPost, "/api/user/recover-by-key", fmt.Sprintf(`{"username":%q,"api_key":%q}`, user.Username, key.Key), "", identity, RecoverAccountByApiKey)
	var challenge struct {
		Success bool `json:"success"`
		Data    struct {
			Required bool   `json:"require_verification"`
			Flow     string `json:"flow_token"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &challenge))
	require.True(t, challenge.Success, response.Body.String())
	require.True(t, challenge.Data.Required)
	require.NotEmpty(t, challenge.Data.Flow)
	assert.NotContains(t, response.Body.String(), "access_token")
	assert.Empty(t, response.Result().Cookies())
	var savedSource, savedTarget model.User
	require.NoError(t, db.First(&savedSource, source.Id).Error)
	require.NoError(t, db.First(&savedTarget, user.Id).Error)
	assert.Equal(t, source.Quota, savedSource.Quota)
	assert.Equal(t, source.DiscordId, savedSource.DiscordId)
	assert.Empty(t, savedTarget.DiscordId)
	assert.Zero(t, savedTarget.Quota)
	// Factor verification uses the existing shared callback and remains single use.
	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)
	body := fmt.Sprintf(`{"flow_token":%q,"code":%q}`, challenge.Data.Flow, code)
	completed := securityEnrollmentRequest(http.MethodPost, "/api/user/login/verify", body, "", service.AuthIdentity{}, VerifyLogin)
	assert.Contains(t, completed.Body.String(), `"success":true`)
	assert.Contains(t, completed.Body.String(), `"login_method":"key_recovery"`)
	assert.Contains(t, completed.Body.String(), "access_token")
	replay := securityEnrollmentRequest(http.MethodPost, "/api/user/login/verify", body, "", service.AuthIdentity{}, VerifyLogin)
	assert.Contains(t, replay.Body.String(), `"success":false`)
}

func TestRecoveryPreservesUnprotectedMigration(t *testing.T) {
	db, user, key := setupRecoveryTest(t)
	source := model.User{Username: "temporary-discord", DiscordId: "123456789012345678", Quota: 12345, Status: common.UserStatusEnabled, Role: common.RoleCommonUser, Group: "default", AuthVersion: 1, AffCode: "source"}
	require.NoError(t, db.Create(&source).Error)
	bundle, err := service.CreateLoginSession(source.Id, "oauth:discord", "127.0.0.1", "recovery-test")
	require.NoError(t, err)
	identity, err := service.ParseAccessToken(bundle.AccessToken)
	require.NoError(t, err)
	response := securityEnrollmentRequest(http.MethodPost, "/api/user/recover-by-key", fmt.Sprintf(`{"username":%q,"api_key":%q}`, user.Username, key.Key), "", identity, RecoverAccountByApiKey)
	require.Contains(t, response.Body.String(), `"success":true`, response.Body.String())
	assert.Contains(t, response.Body.String(), "access_token")
	var saved model.User
	require.NoError(t, db.First(&saved, user.Id).Error)
	assert.Equal(t, source.DiscordId, saved.DiscordId)
	assert.Equal(t, source.Quota, saved.Quota)
	assert.False(t, strings.Contains(response.Body.String(), key.Key))
}
