package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetPaginatedChannelGroupsSplitsDeduplicatesAndPaginates(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Create(&[]Channel{
		{Name: "first", Key: "key-1", Group: "vip,default"},
		{Name: "second", Key: "key-2", Group: "vip,beta"},
	}).Error)

	groups, total, err := GetPaginatedChannelGroups(DB.Model(&Channel{}), 1, 2)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Equal(t, []string{"default", "vip"}, groups)
}
