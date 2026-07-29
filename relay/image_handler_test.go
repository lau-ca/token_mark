package relay

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCompositeImageRequestDisablesBodyPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	globalSettings := model_setting.GetGlobalSettings()
	originalGlobalPassthrough := globalSettings.PassThroughRequestEnabled
	t.Cleanup(func() {
		globalSettings.PassThroughRequestEnabled = originalGlobalPassthrough
	})

	globalSettings.PassThroughRequestEnabled = false
	assert.True(t, shouldPassThroughImageRequest(c, true))

	globalSettings.PassThroughRequestEnabled = true
	assert.True(t, shouldPassThroughImageRequest(c, false))

	common.SetContextKey(c, constant.ContextKeyCompositeDisableRequestBodyPassthrough, true)
	assert.False(t, shouldPassThroughImageRequest(c, true))
	assert.False(t, shouldPassThroughImageRequest(c, false))
}
