package service

import (
	"math"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const modelSuccessRateCacheTTL = 15 * time.Second

type ModelSuccessRateResponse struct {
	Model        string                 `json:"model"`
	Timezone     string                 `json:"timezone"`
	GeneratedAt  int64                  `json:"generated_at"`
	Today        ModelSuccessRateWindow `json:"today"`
	Last5Minutes ModelSuccessRateWindow `json:"last_5_minutes"`
}

type ModelSuccessRateWindow struct {
	ErrorRateIncluding4xx *float64 `json:"error_rate_including_4xx"`
	ErrorRateExcluding4xx *float64 `json:"error_rate_excluding_4xx"`
}

func GetModelSuccessRate(modelName string) (ModelSuccessRateResponse, error) {
	cacheKey := "model_success_rate:" + common.Sha1([]byte(modelName))
	if common.RedisEnabled && common.RDB != nil {
		if cached, err := common.RedisGet(cacheKey); err == nil {
			var response ModelSuccessRateResponse
			if common.UnmarshalJsonStr(cached, &response) == nil {
				return response, nil
			}
		}
	}

	response, err := getModelSuccessRateAt(modelName, time.Now())
	if err != nil {
		return ModelSuccessRateResponse{}, err
	}
	if common.RedisEnabled && common.RDB != nil {
		if data, marshalErr := common.Marshal(response); marshalErr == nil {
			_ = common.RedisSet(cacheKey, string(data), modelSuccessRateCacheTTL)
		}
	}
	return response, nil
}

func getModelSuccessRateAt(modelName string, now time.Time) (ModelSuccessRateResponse, error) {
	generatedAt := now.Unix()
	_, todayStart, _ := model.BeijingDayRange(now)
	last5MinutesStart := generatedAt - int64((5*time.Minute)/time.Second)

	todayCounts, err := model.GetModelSuccessRateCounts(modelName, todayStart, generatedAt)
	if err != nil {
		return ModelSuccessRateResponse{}, err
	}
	last5MinutesCounts, err := model.GetModelSuccessRateCounts(modelName, last5MinutesStart, generatedAt)
	if err != nil {
		return ModelSuccessRateResponse{}, err
	}

	return ModelSuccessRateResponse{
		Model:        modelName,
		Timezone:     "Asia/Shanghai",
		GeneratedAt:  generatedAt,
		Today:        newModelSuccessRateWindow(todayCounts),
		Last5Minutes: newModelSuccessRateWindow(last5MinutesCounts),
	}, nil
}

func newModelSuccessRateWindow(counts model.ModelSuccessRateCounts) ModelSuccessRateWindow {
	requestCount := counts.SuccessCount + counts.ClientErrorCount + counts.ServerErrorCount + counts.OtherErrorCount
	nonClientRequestCount := counts.SuccessCount + counts.ServerErrorCount + counts.OtherErrorCount
	return ModelSuccessRateWindow{
		ErrorRateIncluding4xx: percentage(counts.ClientErrorCount+counts.ServerErrorCount+counts.OtherErrorCount, requestCount),
		ErrorRateExcluding4xx: percentage(counts.ServerErrorCount+counts.OtherErrorCount, nonClientRequestCount),
	}
}

func percentage(numerator, denominator int64) *float64 {
	if denominator == 0 {
		return nil
	}
	value := math.Round(float64(numerator)*100*10000/float64(denominator)) / 10000
	return &value
}
