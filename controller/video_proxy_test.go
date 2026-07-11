package controller

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	videoProxyTestTaskID         = "task_public_video"
	videoProxyTestUpstreamTaskID = "upstream-video-id"
	videoProxyTestUserID         = 101
)

type videoProxyTestOptions struct {
	channelType               int
	replaceVideoURLsWithProxy bool
	otherSettings             string
	baseURL                   string
	resultURL                 string
	userID                    int
}

func setupVideoProxyTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalRedisEnabled := common.RedisEnabled
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()
	originalFetchSetting := *system_setting.GetFetchSetting()

	common.MemoryCacheEnabled = false
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	system_setting.GetFetchSetting().EnableSSRFProtection = false
	service.InitHttpClient()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Task{}))

	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.RedisEnabled = originalRedisEnabled
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		*system_setting.GetFetchSetting() = originalFetchSetting
		service.InitHttpClient()
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedVideoProxyTest(t *testing.T, db *gorm.DB, options videoProxyTestOptions) {
	t.Helper()

	userID := options.userID
	if userID == 0 {
		userID = videoProxyTestUserID
	}
	settings := options.otherSettings
	if settings == "" {
		settings = fmt.Sprintf(`{"replace_video_urls_with_proxy":%t}`, options.replaceVideoURLsWithProxy)
	}
	baseURL := options.baseURL
	channel := &model.Channel{
		Id:            1,
		Type:          options.channelType,
		Key:           "upstream-secret-key",
		Status:        common.ChannelStatusEnabled,
		Name:          "video proxy test",
		BaseURL:       &baseURL,
		Models:        "video-test-model",
		Group:         "default",
		OtherSettings: settings,
	}
	require.NoError(t, db.Create(channel).Error)

	task := &model.Task{
		TaskID:    videoProxyTestTaskID,
		UserId:    userID,
		ChannelId: channel.Id,
		Status:    model.TaskStatusSuccess,
		Progress:  "100%",
		Data:      json.RawMessage(`{}`),
		Properties: model.Properties{
			OriginModelName: "video-test-model",
		},
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: videoProxyTestUpstreamTaskID,
			ResultURL:      options.resultURL,
		},
	}
	require.NoError(t, db.Create(task).Error)
}

func performVideoProxyRequest(t *testing.T, userID int, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/"+videoProxyTestTaskID+"/content", nil)
	for key, value := range headers {
		c.Request.Header.Set(key, value)
	}
	c.Params = gin.Params{{Key: "task_id", Value: videoProxyTestTaskID}}
	c.Set("id", userID)

	VideoProxy(c)
	return recorder
}

func TestVideoProxyPrivacyForwardsRangesAndFiltersResponseHeaders(t *testing.T) {
	for _, channelType := range []int{constant.ChannelTypeOpenAI, constant.ChannelTypeSora} {
		t.Run(fmt.Sprintf("channel_type_%d", channelType), func(t *testing.T) {
			db := setupVideoProxyTestDB(t)
			var upstreamHeaders http.Header
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upstreamHeaders = r.Header.Clone()
				w.Header().Set("Content-Type", "video/mp4")
				w.Header().Set("Content-Length", "4")
				w.Header().Set("Content-Range", "bytes 2-5/10")
				w.Header().Set("Accept-Ranges", "bytes")
				w.Header().Set("Content-Disposition", `inline; filename="result.mp4"`)
				w.Header().Set("ETag", `"video-v1"`)
				w.Header().Set("Last-Modified", "Fri, 10 Jul 2026 12:00:00 GMT")
				w.Header().Set("Location", "https://storage.example.com/signed.mp4?token=secret")
				w.Header().Set("Content-Location", "https://storage.example.com/private.mp4")
				w.Header().Set("Link", "<https://storage.example.com/private>; rel=canonical")
				w.Header().Set("Refresh", "0;url=https://storage.example.com/private")
				w.Header().Set("Set-Cookie", "upstream_session=secret")
				w.Header().Set("Server", "upstream-private-server")
				w.Header().Set("X-Upstream-Secret", "secret")
				w.Header().Set("Cache-Control", "public, max-age=86400")
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write([]byte("2345"))
			}))
			defer server.Close()

			seedVideoProxyTest(t, db, videoProxyTestOptions{
				channelType:               channelType,
				replaceVideoURLsWithProxy: true,
				baseURL:                   server.URL,
			})
			recorder := performVideoProxyRequest(t, videoProxyTestUserID, map[string]string{
				"Range":    "bytes=2-5",
				"If-Range": `"video-v1"`,
			})

			require.NotNil(t, upstreamHeaders)
			assert.Equal(t, "bytes=2-5", upstreamHeaders.Get("Range"))
			assert.Equal(t, `"video-v1"`, upstreamHeaders.Get("If-Range"))
			assert.Equal(t, "identity", upstreamHeaders.Get("Accept-Encoding"))
			assert.Equal(t, "Bearer upstream-secret-key", upstreamHeaders.Get("Authorization"))

			require.Equal(t, http.StatusPartialContent, recorder.Code)
			assert.Equal(t, "2345", recorder.Body.String())
			assert.Equal(t, "video/mp4", recorder.Header().Get("Content-Type"))
			assert.Equal(t, "4", recorder.Header().Get("Content-Length"))
			assert.Equal(t, "bytes 2-5/10", recorder.Header().Get("Content-Range"))
			assert.Equal(t, "bytes", recorder.Header().Get("Accept-Ranges"))
			assert.Equal(t, `inline; filename="result.mp4"`, recorder.Header().Get("Content-Disposition"))
			assert.Equal(t, `"video-v1"`, recorder.Header().Get("ETag"))
			assert.Equal(t, "Fri, 10 Jul 2026 12:00:00 GMT", recorder.Header().Get("Last-Modified"))
			assert.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
			for _, key := range []string{
				"Location",
				"Content-Location",
				"Link",
				"Refresh",
				"Set-Cookie",
				"Server",
				"X-Upstream-Secret",
				"Date",
			} {
				assert.Empty(t, recorder.Header().Values(key), key)
			}
		})
	}
}

func TestSeedanceVideoProxyFollowsRedirectWithoutLeakingCredentials(t *testing.T) {
	db := setupVideoProxyTestDB(t)
	var contentHeaders http.Header
	contentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Range", "bytes 0-3/10")
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("video"))
	}))
	defer contentServer.Close()

	var upstreamAuthorization string
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamAuthorization = r.Header.Get("Authorization")
		http.Redirect(w, r, contentServer.URL+"/content", http.StatusFound)
	}))
	defer upstreamServer.Close()

	seedVideoProxyTest(t, db, videoProxyTestOptions{
		channelType: constant.ChannelTypeSeedance,
		baseURL:     upstreamServer.URL,
	})
	recorder := performVideoProxyRequest(t, videoProxyTestUserID, map[string]string{
		"Range":    "bytes=0-3",
		"If-Range": `"video-v1"`,
	})

	assert.Equal(t, "Bearer upstream-secret-key", upstreamAuthorization)
	require.NotNil(t, contentHeaders)
	assert.Empty(t, contentHeaders.Get("Authorization"))
	assert.Equal(t, "bytes=0-3", contentHeaders.Get("Range"))
	assert.Equal(t, `"video-v1"`, contentHeaders.Get("If-Range"))
	require.Equal(t, http.StatusPartialContent, recorder.Code)
	assert.Equal(t, "video", recorder.Body.String())
	assert.Equal(t, "video/mp4", recorder.Header().Get("Content-Type"))
	assert.Empty(t, recorder.Header().Get("Location"))
	assert.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
}

func TestSeedanceVideoProxyRejectsNonVideoContent(t *testing.T) {
	db := setupVideoProxyTestDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("not a video"))
	}))
	defer server.Close()

	seedVideoProxyTest(t, db, videoProxyTestOptions{
		channelType: constant.ChannelTypeSeedance,
		baseURL:     server.URL,
	})
	recorder := performVideoProxyRequest(t, videoProxyTestUserID, nil)

	assert.Equal(t, http.StatusBadGateway, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), server.URL)
}

func TestVideoProxyPrivacyPropagatesRangeNotSatisfiable(t *testing.T) {
	db := setupVideoProxyTestDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Range", "bytes */10")
		w.Header().Set("Location", "https://storage.example.com/private")
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		_, _ = w.Write([]byte("upstream private error"))
	}))
	defer server.Close()

	seedVideoProxyTest(t, db, videoProxyTestOptions{
		channelType:               constant.ChannelTypeOpenAI,
		replaceVideoURLsWithProxy: true,
		baseURL:                   server.URL,
	})
	recorder := performVideoProxyRequest(t, videoProxyTestUserID, map[string]string{
		"Range": "bytes=99-100",
	})

	require.Equal(t, http.StatusRequestedRangeNotSatisfiable, recorder.Code)
	assert.Equal(t, "bytes */10", recorder.Header().Get("Content-Range"))
	assert.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
	assert.Empty(t, recorder.Header().Get("Location"))
	assert.Empty(t, recorder.Body.String())
}

func TestVideoProxyPrivacyDoesNotFollowOrExposeRedirect(t *testing.T) {
	db := setupVideoProxyTestDB(t)
	redirectFollowed := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect-target" {
			redirectFollowed = true
			_, _ = w.Write([]byte("private redirect body"))
			return
		}
		w.Header().Set("Location", "/redirect-target?signature=secret")
		w.Header().Set("X-Upstream-Secret", "secret")
		w.WriteHeader(http.StatusFound)
		_, _ = w.Write([]byte("upstream redirect body"))
	}))
	defer server.Close()

	seedVideoProxyTest(t, db, videoProxyTestOptions{
		channelType:               constant.ChannelTypeSora,
		replaceVideoURLsWithProxy: true,
		baseURL:                   server.URL,
	})
	recorder := performVideoProxyRequest(t, videoProxyTestUserID, nil)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	assert.False(t, redirectFollowed)
	assert.Empty(t, recorder.Header().Get("Location"))
	assert.Empty(t, recorder.Header().Get("X-Upstream-Secret"))
	assert.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
	assert.NotContains(t, recorder.Body.String(), "signature=secret")
	assert.NotContains(t, recorder.Body.String(), "upstream redirect body")
}

func TestVideoProxyPrivacyDataURLSupportsRange(t *testing.T) {
	db := setupVideoProxyTestDB(t)
	videoData := []byte("0123456789")
	dataURL := "data:video/mp4;base64," + base64.StdEncoding.EncodeToString(videoData)
	seedVideoProxyTest(t, db, videoProxyTestOptions{
		channelType:               constant.ChannelTypeOpenAI,
		replaceVideoURLsWithProxy: true,
		baseURL:                   "https://unused.example.com",
		resultURL:                 dataURL,
	})
	recorder := performVideoProxyRequest(t, videoProxyTestUserID, map[string]string{
		"Range": "bytes=2-5",
	})

	require.Equal(t, http.StatusPartialContent, recorder.Code)
	assert.Equal(t, "2345", recorder.Body.String())
	assert.Equal(t, "bytes 2-5/10", recorder.Header().Get("Content-Range"))
	assert.Equal(t, "4", recorder.Header().Get("Content-Length"))
	assert.Equal(t, "bytes", recorder.Header().Get("Accept-Ranges"))
	assert.Equal(t, "video/mp4", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
}

func TestVideoProxyToggleOffPreservesLegacyBehavior(t *testing.T) {
	db := setupVideoProxyTestDB(t)
	redirectFollowed := false
	var firstRequestRange string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/legacy-target" {
			redirectFollowed = true
			w.Header().Set("X-Legacy-Header", "exposed")
			w.Header().Set("Location", "https://storage.example.com/legacy")
			w.Header().Set("Cache-Control", "private")
			_, _ = w.Write([]byte("legacy body"))
			return
		}
		firstRequestRange = r.Header.Get("Range")
		http.Redirect(w, r, "/legacy-target", http.StatusFound)
	}))
	defer server.Close()

	seedVideoProxyTest(t, db, videoProxyTestOptions{
		channelType:               constant.ChannelTypeOpenAI,
		replaceVideoURLsWithProxy: false,
		baseURL:                   server.URL,
	})
	recorder := performVideoProxyRequest(t, videoProxyTestUserID, map[string]string{
		"Range": "bytes=2-5",
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.True(t, redirectFollowed)
	assert.Empty(t, firstRequestRange)
	assert.Equal(t, "legacy body", recorder.Body.String())
	assert.Equal(t, "exposed", recorder.Header().Get("X-Legacy-Header"))
	assert.Equal(t, "https://storage.example.com/legacy", recorder.Header().Get("Location"))
	assert.Equal(t, "public, max-age=86400", recorder.Header().Get("Cache-Control"))
}

func TestVideoProxyUnsupportedChannelPreservesLegacyBehavior(t *testing.T) {
	db := setupVideoProxyTestDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Legacy-Header", "exposed")
		w.Header().Set("Location", "https://storage.example.com/legacy")
		_, _ = w.Write([]byte("legacy body"))
	}))
	defer server.Close()

	seedVideoProxyTest(t, db, videoProxyTestOptions{
		channelType:               constant.ChannelTypeXai,
		replaceVideoURLsWithProxy: true,
		baseURL:                   "https://unused.example.com",
		resultURL:                 server.URL,
	})
	recorder := performVideoProxyRequest(t, videoProxyTestUserID, nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "legacy body", recorder.Body.String())
	assert.Equal(t, "exposed", recorder.Header().Get("X-Legacy-Header"))
	assert.Equal(t, "https://storage.example.com/legacy", recorder.Header().Get("Location"))
	assert.Equal(t, "public, max-age=86400", recorder.Header().Get("Cache-Control"))
}

func TestVideoProxyMalformedSupportedSettingsFailWithoutMutation(t *testing.T) {
	db := setupVideoProxyTestDB(t)
	upstreamCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalled = true
		_, _ = w.Write([]byte("private video"))
	}))
	defer server.Close()

	const malformedSettings = "malformed settings must remain untouched"
	seedVideoProxyTest(t, db, videoProxyTestOptions{
		channelType:   constant.ChannelTypeOpenAI,
		otherSettings: malformedSettings,
		baseURL:       server.URL,
	})
	recorder := performVideoProxyRequest(t, videoProxyTestUserID, nil)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.False(t, upstreamCalled)
	var storedChannel model.Channel
	require.NoError(t, db.First(&storedChannel, 1).Error)
	assert.Equal(t, malformedSettings, storedChannel.OtherSettings)
}

func TestVideoProxyPreservesTaskOwnership(t *testing.T) {
	db := setupVideoProxyTestDB(t)
	upstreamCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalled = true
		_, _ = w.Write([]byte("private video"))
	}))
	defer server.Close()

	seedVideoProxyTest(t, db, videoProxyTestOptions{
		channelType:               constant.ChannelTypeOpenAI,
		replaceVideoURLsWithProxy: true,
		baseURL:                   server.URL,
		userID:                    videoProxyTestUserID,
	})
	recorder := performVideoProxyRequest(t, videoProxyTestUserID+1, nil)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	assert.False(t, upstreamCalled)
}
