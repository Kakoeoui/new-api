package model

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"io"
	"math/big"
	"net/http"
	"slices"
	"strconv"

	"github.com/QuantumNous/new-api/common"
)

const (
	CheckinDiscordGuildID       = "1444925702234177640"
	CheckinDiscordMinimumRoleID = "1509198060599902279"
)

// Each band has its own probability; amounts inside a band are equally likely.
func drawCheckinQuota(minQuota, maxQuota int, random io.Reader) (int, error) {
	if minQuota < 15000 || maxQuota > 250000 || minQuota > maxQuota {
		return 0, common.NewMessage("Check-in reward range must be between 0.03 and 0.50 THB")
	}
	bands := []struct{ min, max, weight int }{
		{3, 8, 80}, {9, 15, 15}, {16, 30, 4}, {31, 50, 1},
	}
	minCents, maxCents := (minQuota+4999)/5000, maxQuota/5000
	total := 0
	for i := range bands {
		bands[i].min = max(bands[i].min, minCents)
		bands[i].max = min(bands[i].max, maxCents)
		if bands[i].min <= bands[i].max {
			total += bands[i].weight
		}
	}
	if total == 0 {
		return 0, common.NewMessage("Check-in reward range must include a whole satang amount")
	}
	ticket, err := rand.Int(random, big.NewInt(int64(total)))
	if err != nil {
		return 0, err
	}
	roll := int(ticket.Int64())
	for _, band := range bands {
		if band.min > band.max {
			continue
		}
		if roll >= band.weight {
			roll -= band.weight
			continue
		}
		amount, err := rand.Int(random, big.NewInt(int64(band.max-band.min+1)))
		if err != nil {
			return 0, err
		}
		return (band.min + int(amount.Int64())) * 5000, nil
	}
	return 0, common.NewMessage("Check-in failed, please try again later")
}

// Fetch current membership for each claim; never trust browser-supplied roles.
func verifyCheckinDiscordMember(ctx context.Context, client *http.Client, discordID, token string) error {
	unavailable := common.NewMessage("Discord verification is unavailable. Please try again later.")
	denied := common.NewMessage("Daily check-in requires membership in our Discord server and the minimum required role or higher.")
	if token == "" {
		return unavailable
	}
	if id, err := strconv.ParseUint(discordID, 10, 64); err != nil || id == 0 {
		return denied
	}
	var member struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
		Roles   []string `json:"roles"`
		Pending bool     `json:"pending"`
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/v10/users/@me/guilds/"+CheckinDiscordGuildID+"/member", nil)
	if err != nil {
		return unavailable
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "RVL-Checkin/1.0")
	res, err := client.Do(req)
	if err != nil {
		return unavailable
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return denied
	}
	if res.StatusCode != http.StatusOK {
		return unavailable
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&member); err != nil {
		return unavailable
	}
	if member.User.ID != discordID || member.Pending {
		return denied
	}
	allowed := operation_setting.GetCheckinSetting().AllowedDiscordRoleIDs
	for _, assignedID := range member.Roles {
		if assignedID == CheckinDiscordMinimumRoleID || slices.Contains(allowed, assignedID) {
			return nil
		}
	}
	return denied
}
