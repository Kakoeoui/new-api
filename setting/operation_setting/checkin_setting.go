package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// CheckinSetting 签到功能配置
type CheckinSetting struct {
	// Approved role snapshot from 2026-11-11; update when the guild hierarchy changes.
	AllowedDiscordRoleIDs []string `json:"allowed_discord_role_ids"`
	Enabled               bool     `json:"enabled"`   // 是否启用签到功能
	MinQuota              int      `json:"min_quota"` // 签到最小额度奖励
	MaxQuota              int      `json:"max_quota"` // 签到最大额度奖励
}

// 500000 quota = 1 THB; rewards range from 0.03 to 0.50 THB.
var checkinSetting = CheckinSetting{
	Enabled:               true,
	AllowedDiscordRoleIDs: []string{"1445147267123253500", "1445181406459465882", "1445323815784681577", "1445408360030539776", "1445412068592451607", "1445453023181541542", "1445468924534853653", "1445812260902670368", "1474630597769953321", "1475418985288699914", "1484402047083352256", "1485100279916531805", "1487526545546547211", "1487529194534735903", "1487529310855106643", "1492197847184183459", "1498380459913248789", "1505746618270617651", "1506708085128036445", "1509197559330111579", "1509198060599902279", "1527129536867860573", "1533783662817050715", "1533892462303514736", "1547454314849509386", "1553507677475180734", "1561698372606361645", "1561699033674678295", "1561699843905421332", "1561700347473559572", "1561700849472897107", "1561701232274440214", "1561701866633437196", "1567200224320168167"},
	MinQuota:              15000,
	MaxQuota:              250000,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("checkin_setting", &checkinSetting)
}

// GetCheckinSetting 获取签到配置
func GetCheckinSetting() *CheckinSetting {
	return &checkinSetting
}

// IsCheckinEnabled 是否启用签到功能
func IsCheckinEnabled() bool {
	return checkinSetting.Enabled
}

// GetCheckinQuotaRange 获取签到额度范围
func GetCheckinQuotaRange() (min, max int) {
	return checkinSetting.MinQuota, checkinSetting.MaxQuota
}
