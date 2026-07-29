package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const maxQuotaForecastUserIDs = 100

type quotaForecastRequest struct {
	UserIDs []int `json:"user_ids"`
}

func normalizeQuotaForecastUserIDs(userIDs []int) ([]int, string) {
	unique := make([]int, 0, len(userIDs))
	seen := make(map[int]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if userID <= 0 {
			return nil, "user_ids must contain positive integers"
		}
		if _, exists := seen[userID]; exists {
			continue
		}
		seen[userID] = struct{}{}
		unique = append(unique, userID)
		if len(unique) > maxQuotaForecastUserIDs {
			return nil, "user_ids cannot contain more than 100 unique users"
		}
	}
	if len(unique) == 0 {
		return nil, "user_ids is required"
	}
	return unique, ""
}

func GetQuotaForecasts(c *gin.Context) {
	request := quotaForecastRequest{}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request body"})
		return
	}
	userIDs, validationMessage := normalizeQuotaForecastUserIDs(request.UserIDs)
	if validationMessage != "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": validationMessage})
		return
	}
	results, err := service.GetQuotaForecasts(c.Request.Context(), userIDs, common.GetTimestamp())
	if err != nil {
		logger.LogError(c.Request.Context(), "failed to calculate quota forecasts: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to calculate quota forecasts"})
		return
	}
	common.ApiSuccess(c, results)
}

func GetSelfQuotaForecast(c *gin.Context) {
	userID := c.GetInt("id")
	if userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "invalid user"})
		return
	}
	results, err := service.GetQuotaForecasts(c.Request.Context(), []int{userID}, common.GetTimestamp())
	if err != nil {
		logger.LogError(c.Request.Context(), "failed to calculate self quota forecast: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to calculate quota forecast"})
		return
	}
	if len(results) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "user not found"})
		return
	}
	common.ApiSuccess(c, results[0])
}
