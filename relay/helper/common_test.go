package helper

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type failingResponseWriter struct {
	gin.ResponseWriter
	err error
}

func (w *failingResponseWriter) Write(_ []byte) (int, error) {
	return 0, w.err
}

func TestClaudeChunkDataReturnsWriteError(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	expectedErr := errors.New("client connection closed")
	c.Writer = &failingResponseWriter{
		ResponseWriter: c.Writer,
		err:            expectedErr,
	}

	err := ClaudeChunkData(c, dto.ClaudeResponse{Type: "message_stop"}, `{"type":"message_stop"}`)

	require.ErrorIs(t, err, expectedErr)
}
