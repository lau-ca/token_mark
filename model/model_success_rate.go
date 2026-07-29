package model

import (
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/common"
)

type ModelSuccessRateCounts struct {
	SuccessCount     int64 `json:"success_count"`
	ClientErrorCount int64 `json:"client_error_count"`
	ServerErrorCount int64 `json:"server_error_count"`
	OtherErrorCount  int64 `json:"other_error_count"`
}

type modelFailedRequest struct {
	Other string `gorm:"column:other"`
}

func GetModelSuccessRateCounts(modelName string, startTimestamp, endTimestamp int64) (ModelSuccessRateCounts, error) {
	counts := ModelSuccessRateCounts{}
	if LOG_DB == nil {
		return counts, fmt.Errorf("log database is not initialized")
	}

	err := LOG_DB.Model(&Log{}).
		Where("model_name = ?", modelName).
		Where("type = ?", LogTypeConsume).
		Where("request_id <> ''").
		Where("created_at >= ? AND created_at <= ?", startTimestamp, endTimestamp).
		Distinct("request_id").
		Count(&counts.SuccessCount).Error
	if err != nil {
		return counts, err
	}

	failedRequests := make([]modelFailedRequest, 0)
	err = LOG_DB.Table("logs AS failed").
		Select("failed.other").
		Where("failed.model_name = ?", modelName).
		Where("failed.type = ?", LogTypeError).
		Where("failed.request_id <> ''").
		Where("failed.created_at >= ? AND failed.created_at <= ?", startTimestamp, endTimestamp).
		Where(`NOT EXISTS (
			SELECT 1 FROM logs AS consumed
			WHERE consumed.model_name = failed.model_name
				AND consumed.request_id = failed.request_id
				AND consumed.type = ?
				AND consumed.created_at >= ? AND consumed.created_at <= ?
		)`, LogTypeConsume, startTimestamp, endTimestamp).
		Where(`NOT EXISTS (
			SELECT 1 FROM logs AS newer
			WHERE newer.model_name = failed.model_name
				AND newer.request_id = failed.request_id
				AND newer.type = ?
				AND newer.created_at >= ? AND newer.created_at <= ?
				AND (newer.created_at > failed.created_at OR (newer.created_at = failed.created_at AND newer.id > failed.id))
		)`, LogTypeError, startTimestamp, endTimestamp).
		Scan(&failedRequests).Error
	if err != nil {
		return counts, err
	}

	for _, failedRequest := range failedRequests {
		statusCode := getLogStatusCode(failedRequest.Other)
		switch {
		case statusCode >= 400 && statusCode <= 499:
			counts.ClientErrorCount++
		case statusCode >= 500 && statusCode <= 599:
			counts.ServerErrorCount++
		default:
			counts.OtherErrorCount++
		}
	}
	return counts, nil
}

func getLogStatusCode(other string) int {
	if other == "" {
		return 0
	}
	var fields map[string]interface{}
	if err := common.UnmarshalJsonStr(other, &fields); err != nil {
		return 0
	}
	switch value := fields["status_code"].(type) {
	case float64:
		return int(value)
	case string:
		statusCode, _ := strconv.Atoi(value)
		return statusCode
	default:
		return 0
	}
}
