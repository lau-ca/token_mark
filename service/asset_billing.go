package service

import (
	"fmt"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
)

func LogAssetConsumption(c *gin.Context, info *relaycommon.RelayInfo, action string) {
	other := map[string]interface{}{
		"asset_action": action,
		"group_ratio":  info.PriceData.GroupRatioInfo.GroupRatio,
		"is_asset_api": true,
		"model_price":  info.PriceData.ModelPrice,
		"request_path": c.Request.URL.Path,
	}
	if info.PriceData.GroupRatioInfo.HasSpecialRatio {
		other["user_group_ratio"] = info.PriceData.GroupRatioInfo.GroupSpecialRatio
	}
	attachQuotaSaturation(c, info, other)
	model.RecordConsumeLog(c, info.UserId, model.RecordConsumeLogParams{
		ModelName: info.OriginModelName,
		TokenName: c.GetString("token_name"),
		Quota:     info.PriceData.Quota,
		Content:   fmt.Sprintf("素材库操作 %s，按次计费", action),
		TokenId:   info.TokenId,
		Group:     info.UsingGroup,
		Other:     other,
	})
	model.UpdateUserUsedQuotaAndRequestCount(info.UserId, info.PriceData.Quota)
}
