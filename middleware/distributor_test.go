package middleware

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetModelRequestRecognizesNativeSeedanceCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v3/contents/generations/tasks", bytes.NewBufferString(`{"model":"doubao-seedance-2-0-260128","content":[{"type":"text","text":"animate"}]}`))
	context.Request.Header.Set("Content-Type", "application/json")

	request, shouldSelectChannel, err := getModelRequest(context)

	require.NoError(t, err)
	assert.True(t, shouldSelectChannel)
	assert.Equal(t, "doubao-seedance-2-0-260128", request.Model)
	assert.Equal(t, relayconstant.RelayModeVideoSubmit, context.GetInt("relay_mode"))
}

func TestGetModelRequestUsesAssetBillingModelForVolcArk(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/volc/ark?Action=CreateAsset", bytes.NewBufferString(`{"Id":"zw-1"}`))
	context.Request.Header.Set("Content-Type", "application/json")

	request, shouldSelectChannel, err := getModelRequest(context)

	require.NoError(t, err)
	assert.True(t, shouldSelectChannel)
	assert.Equal(t, constant.SeedanceAssetBillingModel, request.Model)
}

func TestGetModelRequestReadsPlaygroundImageEditMultipart(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	require.NoError(t, writer.WriteField("group", "codex_image"))
	file, err := writer.CreateFormFile("image", "reference.png")
	require.NoError(t, err)
	_, err = file.Write([]byte("image-data"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/pg/images/edits", &body)
	context.Request.Header.Set("Content-Type", writer.FormDataContentType())

	request, shouldSelectChannel, err := getModelRequest(context)

	require.NoError(t, err)
	require.NotNil(t, request)
	assert.True(t, shouldSelectChannel)
	assert.Equal(t, "gpt-image-2", request.Model)
	assert.Equal(t, "codex_image", request.Group)
}
