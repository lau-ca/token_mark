package xai

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertImageRequestAspectRatio(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		aspectRatioJSON json.RawMessage
		want            string
	}{
		{name: "landscape widescreen", aspectRatioJSON: json.RawMessage(`"16:9"`), want: "16:9"},
		{name: "portrait widescreen", aspectRatioJSON: json.RawMessage(`"9:16"`), want: "9:16"},
		{name: "landscape standard", aspectRatioJSON: json.RawMessage(`"4:3"`), want: "4:3"},
		{name: "portrait standard", aspectRatioJSON: json.RawMessage(`"3:4"`), want: "3:4"},
		{name: "empty string", aspectRatioJSON: json.RawMessage(`""`)},
		{name: "non string", aspectRatioJSON: json.RawMessage(`16`)},
		{name: "missing"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := dto.ImageRequest{
				Model:  "grok-imagine-image",
				Prompt: "a geometric landscape",
			}
			if test.aspectRatioJSON != nil {
				request.Extra = map[string]json.RawMessage{
					"aspect_ratio": test.aspectRatioJSON,
				}
			}

			converted, err := (&Adaptor{}).ConvertImageRequest(
				gin.CreateTestContextOnly(httptest.NewRecorder(), gin.New()),
				nil,
				request,
			)
			require.NoError(t, err)

			xaiRequest, ok := converted.(ImageRequest)
			require.True(t, ok)
			assert.Equal(t, test.want, xaiRequest.AspectRatio)

			body, err := common.Marshal(xaiRequest)
			require.NoError(t, err)
			var payload map[string]any
			require.NoError(t, common.Unmarshal(body, &payload))
			if test.want == "" {
				assert.NotContains(t, payload, "aspect_ratio")
			} else {
				assert.Equal(t, test.want, payload["aspect_ratio"])
			}
		})
	}
}
