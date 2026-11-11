package model

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckinWeightedRewardBoundaries(t *testing.T) {
	for _, tc := range []struct {
		ticket, offset byte
		quota          int
	}{
		{0, 0, 15000}, {79, 5, 40000}, {80, 0, 45000}, {94, 6, 75000},
		{95, 0, 80000}, {98, 14, 150000}, {99, 0, 155000}, {99, 19, 250000},
	} {
		quota, err := drawCheckinQuota(15000, 250000, bytes.NewReader([]byte{tc.ticket, tc.offset}))
		require.NoError(t, err)
		assert.Equal(t, tc.quota, quota)
	}
	for _, bounds := range [][2]int{{-1, 250000}, {15000, 500000}, {250000, 15000}, {15001, 19999}} {
		_, err := drawCheckinQuota(bounds[0], bounds[1], bytes.NewReader(nil))
		require.Error(t, err)
	}
	quota, err := drawCheckinQuota(15000, 15000, bytes.NewReader([]byte{0, 0}))
	require.NoError(t, err)
	assert.Equal(t, 15000, quota)
	_, err = drawCheckinQuota(15000, 250000, bytes.NewReader(nil))
	require.Error(t, err)
}

type checkinRoundTripper func(*http.Request) (*http.Response, error)

func (f checkinRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCheckinDiscordEligibility(t *testing.T) {
	settings := operation_setting.GetCheckinSetting()
	previous := settings.AllowedDiscordRoleIDs
	settings.AllowedDiscordRoleIDs = []string{"higher"}
	t.Cleanup(func() { settings.AllowedDiscordRoleIDs = previous })
	const userID = "123456789012345678"
	for _, tc := range []struct {
		name, member string
		status       int
		wantErr      bool
	}{
		{"required", `{"user":{"id":"123456789012345678"},"roles":["1509198060599902279"]}`, 200, false},
		{"higher", `{"user":{"id":"123456789012345678"},"roles":["higher"]}`, 200, false},
		{"lower", `{"user":{"id":"123456789012345678"},"roles":["lower"]}`, 200, true},
		{"unapproved role", `{"user":{"id":"123456789012345678"},"roles":["same"]}`, 200, true},
		{"no role", `{"user":{"id":"123456789012345678"},"roles":[]}`, 200, true},
		{"screening pending", `{"user":{"id":"123456789012345678"},"roles":["higher"],"pending":true}`, 200, true},
		{"wrong identity", `{"user":{"id":"other"},"roles":["higher"]}`, 200, true},
		{"non member", `{}`, 404, true},
		{"rate limited", `{}`, 429, true},
		{"unauthorized", `{}`, 401, true},
		{"bad response", `not-json`, 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: checkinRoundTripper(func(r *http.Request) (*http.Response, error) {
				assert.Equal(t, "discord.com", r.URL.Host)
				assert.Equal(t, "/api/v10/users/@me/guilds/1444925702234177640/member", r.URL.Path)
				assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
				body, status := tc.member, tc.status
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}
			err := verifyCheckinDiscordMember(context.Background(), client, userID, "test-token")
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
	client := &http.Client{Transport: checkinRoundTripper(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })}
	require.Error(t, verifyCheckinDiscordMember(context.Background(), client, userID, "test-token"))
	require.Error(t, verifyCheckinDiscordMember(context.Background(), client, userID, ""))
	require.Error(t, verifyCheckinDiscordMember(context.Background(), client, "../invalid", "test-token"))
}

func TestCheckinCreditIsAtomicAndRejectsChangedDiscordIdentity(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Checkin{}))
	user := User{Username: "checkin-credit-policy", Password: "placeholder", DiscordId: "123456789012345678"}
	require.NoError(t, DB.Create(&user).Error)
	t.Cleanup(func() { DB.Where("user_id = ?", user.Id).Delete(&Checkin{}); DB.Unscoped().Delete(&user) })
	claim := &Checkin{UserId: user.Id, CheckinDate: time.Now().Format("2006-01-02"), QuotaAwarded: 15000}
	_, err := userCheckinWithTransaction(claim, user.Id, 15000, "different")
	require.Error(t, err)
	var count int64
	require.NoError(t, DB.Model(&Checkin{}).Where("user_id = ?", user.Id).Count(&count).Error)
	assert.Zero(t, count)
	_, err = userCheckinWithTransaction(claim, user.Id, 15000, user.DiscordId)
	require.NoError(t, err)
	_, err = userCheckinWithTransaction(&Checkin{UserId: user.Id, CheckinDate: claim.CheckinDate, QuotaAwarded: 15000}, user.Id, 15000, user.DiscordId)
	require.Error(t, err)
	var persisted User
	require.NoError(t, DB.First(&persisted, user.Id).Error)
	assert.Equal(t, 15000, persisted.Quota)
}
