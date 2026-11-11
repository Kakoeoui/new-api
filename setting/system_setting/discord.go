package system_setting

import "github.com/QuantumNous/new-api/setting/config"

type DiscordSettings struct {
	Enabled                bool   `json:"enabled"`
	ClientId               string `json:"client_id"`
	ClientSecret           string `json:"client_secret"`
	RequireForRegistration bool   `json:"require_for_registration"`
}

// 默认配置
var defaultDiscordSettings = DiscordSettings{
	Enabled: true,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("discord", &defaultDiscordSettings)
}

func GetDiscordSettings() *DiscordSettings {
	return &defaultDiscordSettings
}
