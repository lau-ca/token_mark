package router

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestVideoRouterRegistersOnfishesCompatibilityRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetVideoRouter(engine)

	routes := make(map[string]struct{})
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, route := range []string{
		"POST /v3/contents/generations/tasks",
		"GET /v3/contents/generations/tasks",
		"GET /v3/contents/generations/tasks/:task_id",
		"DELETE /v3/contents/generations/tasks/:task_id",
		"POST /v1/volc/ark",
		"POST /volc/ark",
	} {
		_, exists := routes[route]
		assert.True(t, exists, route)
	}
}
