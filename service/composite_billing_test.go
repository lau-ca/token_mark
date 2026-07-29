package service

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newCompositeBillingTestDB(t *testing.T, quota int) (*gorm.DB, model.User) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:composite-billing-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	user := model.User{Username: "composite-billing", Password: "password", Quota: quota}
	require.NoError(t, db.Create(&user).Error)
	return db, user
}

func newCompositeBillingSession(t *testing.T, quota int, preConsume int) (*gorm.DB, model.User, *BillingSession) {
	t.Helper()
	db, user := newCompositeBillingTestDB(t, quota)
	originalDB := model.DB
	t.Cleanup(func() {
		model.DB = originalDB
	})
	model.DB = db

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserId:          user.Id,
		IsPlayground:    true,
		ForcePreConsume: true,
		UserSetting:     dto.UserSetting{BillingPreference: "wallet_only"},
	}
	session, apiErr := NewBillingSession(c, info, preConsume)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	return db, user, session
}

func TestCompositeBillingReserveAndSettleUsesSuccessfulTargetQuota(t *testing.T) {
	db, user, session := newCompositeBillingSession(t, 1000, 100)
	require.NoError(t, session.Reserve(250))
	require.NoError(t, session.Settle(150))

	var updated model.User
	require.NoError(t, db.First(&updated, user.Id).Error)
	assert.Equal(t, 850, updated.Quota)
	assert.False(t, session.NeedsRefund())
}

func TestCompositeBillingReserveRejectsInsufficientWalletQuota(t *testing.T) {
	db, user, session := newCompositeBillingSession(t, 300, 100)

	require.Error(t, session.Reserve(400))

	var updated model.User
	require.NoError(t, db.First(&updated, user.Id).Error)
	assert.Equal(t, 200, updated.Quota)
	assert.True(t, session.NeedsRefund())
}

func TestCompositeBillingRefundReturnsAllReservedQuota(t *testing.T) {
	db, user, session := newCompositeBillingSession(t, 1000, 100)
	require.NoError(t, session.Reserve(250))

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	session.Refund(c)

	require.Eventually(t, func() bool {
		var updated model.User
		if err := db.First(&updated, user.Id).Error; err != nil {
			return false
		}
		return updated.Quota == 1000
	}, time.Second, 10*time.Millisecond)
	assert.False(t, session.NeedsRefund())
}
