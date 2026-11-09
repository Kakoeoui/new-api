package controller

import (
	"crypto/subtle"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type recoverAccountByKeyRequest struct {
	Username string `json:"username"`
	ApiKey   string `json:"api_key"`
}

var errInvalidRecoveryCredentials = errors.New("ชื่อผู้ใช้หรือคีย์ API ไม่ถูกต้อง")

// RecoverAccountByApiKey กู้คืนบัญชีด้วยชื่อผู้ใช้และคีย์ API พร้อมย้ายการผูก Discord/โควตา/คีย์จากบัญชีชั่วคราว (ถ้ามี)
func RecoverAccountByApiKey(c *gin.Context) {
	var req recoverAccountByKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, errors.New("กรุณากรอกชื่อผู้ใช้และคีย์ API ให้ครบถ้วน"))
		return
	}

	username := strings.TrimSpace(req.Username)
	rawKey := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(req.ApiKey), "sk-"))
	if username == "" || rawKey == "" {
		common.ApiError(c, errors.New("กรุณากรอกชื่อผู้ใช้และคีย์ API ให้ครบถ้วน"))
		return
	}

	identity, hasSession := middleware.GetSessionAuthIdentity(c)
	targetUser := &model.User{}

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(&model.User{Username: username}).First(targetUser).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errInvalidRecoveryCredentials
			}
			return err
		}
		if targetUser.Id <= 0 || targetUser.Status != common.UserStatusEnabled {
			return errInvalidRecoveryCredentials
		}

		var matchedToken model.Token
		if err := tx.Where(&model.Token{UserId: targetUser.Id, Key: rawKey}).First(&matchedToken).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errInvalidRecoveryCredentials
			}
			return err
		}
		if subtle.ConstantTimeCompare([]byte(matchedToken.Key), []byte(rawKey)) != 1 {
			return errInvalidRecoveryCredentials
		}

		if hasSession && identity.UserID > 0 && identity.UserID != targetUser.Id {
			var currentUser model.User
			err := tx.First(&currentUser, identity.UserID).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err == nil && currentUser.Id > 0 && strings.TrimSpace(currentUser.DiscordId) != "" {
				discordId := strings.TrimSpace(currentUser.DiscordId)

				if err := tx.Unscoped().Model(&model.User{}).
					Where("discord_id = ? AND id <> ?", discordId, targetUser.Id).
					Update("discord_id", "").Error; err != nil {
					return err
				}

				if currentUser.Quota > 0 {
					quotaToTransfer := currentUser.Quota
					if err := tx.Model(&model.User{}).
						Where("id = ?", currentUser.Id).
						Update("quota", 0).Error; err != nil {
						return err
					}
					if err := tx.Model(&model.User{}).
						Where("id = ?", targetUser.Id).
						Update("quota", gorm.Expr("quota + ?", quotaToTransfer)).Error; err != nil {
						return err
					}
				}

				if err := tx.Model(&model.Token{}).
					Where("user_id = ?", currentUser.Id).
					Update("user_id", targetUser.Id).Error; err != nil {
					return err
				}

				if err := tx.Model(&model.User{}).
					Where("id = ?", targetUser.Id).
					Update("discord_id", discordId).Error; err != nil {
					return err
				}

				if currentUser.Password == "" && currentUser.Role == common.RoleCommonUser {
					if err := tx.Model(&model.User{}).
						Where("id = ?", currentUser.Id).
						Update("auth_version", gorm.Expr("auth_version + 1")).Error; err != nil {
						return err
					}
					if err := tx.Delete(&currentUser).Error; err != nil {
						return err
					}
				}
			}
		}

		if err := tx.First(targetUser, targetUser.Id).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errInvalidRecoveryCredentials) {
			common.ApiError(c, err)
			return
		}
		common.SysError(common.LogText("recover account by api key failed: %v", err))
		common.ApiError(c, errors.New("ไม่สามารถกู้คืนบัญชีได้ในขณะนี้ กรุณาลองใหม่อีกครั้ง"))
		return
	}

	if strings.TrimSpace(targetUser.DiscordId) != "" {
		c.Set("login_method", "oauth:discord")
	} else {
		c.Set("login_method", "key_recovery")
	}
	setupLoginAtAuthVersion(targetUser, targetUser.AuthVersion, c)
}
