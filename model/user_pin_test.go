package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserListsPutPinnedUsersFirst(t *testing.T) {
	setupUserUpdateTestState(t)

	users := []User{
		{Username: "unpinned", Password: "password", AffCode: "unpinned", Status: common.UserStatusEnabled},
		{Username: "pinned-older", Password: "password", AffCode: "pinned-older", Status: common.UserStatusEnabled, PinnedAt: 100},
		{Username: "pinned-newer", Password: "password", AffCode: "pinned-newer", Status: common.UserStatusEnabled, PinnedAt: 200},
	}
	require.NoError(t, DB.Create(&users).Error)

	pageInfo := &common.PageInfo{Page: 1, PageSize: 10}
	listedUsers, total, err := GetAllUsers(pageInfo)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.Len(t, listedUsers, 3)
	assert.Equal(t, []string{"pinned-newer", "pinned-older", "unpinned"}, []string{
		listedUsers[0].Username,
		listedUsers[1].Username,
		listedUsers[2].Username,
	})

	searchedUsers, searchTotal, err := SearchUsers("", "", nil, nil, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(3), searchTotal)
	require.Len(t, searchedUsers, 3)
	assert.Equal(t, []string{"pinned-newer", "pinned-older", "unpinned"}, []string{
		searchedUsers[0].Username,
		searchedUsers[1].Username,
		searchedUsers[2].Username,
	})
}

func TestSetUserPinnedPersistsAndClearsZeroValue(t *testing.T) {
	setupUserUpdateTestState(t)

	user := User{
		Username: "pin-toggle",
		Password: "password",
		AffCode:  "pin-toggle",
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, DB.Create(&user).Error)

	pinnedAt, err := SetUserPinned(user.Id, true)
	require.NoError(t, err)
	assert.Positive(t, pinnedAt)

	var stored User
	require.NoError(t, DB.First(&stored, user.Id).Error)
	assert.Equal(t, pinnedAt, stored.PinnedAt)

	pinnedAt, err = SetUserPinned(user.Id, false)
	require.NoError(t, err)
	assert.Zero(t, pinnedAt)
	require.NoError(t, DB.First(&stored, user.Id).Error)
	assert.Zero(t, stored.PinnedAt)
}

func TestUserUpdateDoesNotOverwritePinnedAt(t *testing.T) {
	setupUserUpdateTestState(t)

	user := User{
		Username:    "pin-race",
		Password:    "password",
		AffCode:     "pin-race",
		DisplayName: "before",
		Status:      common.UserStatusEnabled,
		PinnedAt:    100,
	}
	require.NoError(t, DB.Create(&user).Error)

	staleUser, err := GetUserById(user.Id, true)
	require.NoError(t, err)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Update("pinned_at", 0).Error)

	staleUser.DisplayName = "after"
	require.NoError(t, staleUser.Update(false))

	var stored User
	require.NoError(t, DB.First(&stored, user.Id).Error)
	assert.Equal(t, "after", stored.DisplayName)
	assert.Zero(t, stored.PinnedAt)
}

func TestSoftDeleteClearsPinnedUser(t *testing.T) {
	setupUserUpdateTestState(t)

	user := User{
		Username: "pinned-delete",
		Password: "password",
		AffCode:  "pinned-delete",
		Status:   common.UserStatusEnabled,
		PinnedAt: 100,
	}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, user.Delete())

	var stored User
	require.NoError(t, DB.Unscoped().First(&stored, user.Id).Error)
	assert.True(t, stored.DeletedAt.Valid)
	assert.Zero(t, stored.PinnedAt)
}
