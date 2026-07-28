package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func relayCompositeImage(c *gin.Context, relayFormat types.RelayFormat) {
	requestId := c.GetString(common.RequestIdKey)
	var newAPIError *types.NewAPIError
	defer func() {
		if newAPIError == nil {
			return
		}
		logger.LogError(c, fmt.Sprintf("composite image relay error: %s", common.LocalLogPreview(newAPIError.Error())))
		if c.Writer.Written() {
			return
		}
		newAPIError.SetMessage(common.MessageWithRequestId(newAPIError.Error(), requestId))
		c.JSON(newAPIError.StatusCode, gin.H{"error": newAPIError.ToOpenAIError()})
	}()

	policy, operation, ok := service.GetCompositePolicyContext(c)
	if !ok {
		newAPIError = types.NewError(errors.New("composite group policy context is missing"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		return
	}

	request, err := helper.GetAndValidateRequest(c, relayFormat)
	if err != nil {
		statusCode := http.StatusBadRequest
		if common.IsRequestBodyTooLargeError(err) || errors.Is(err, common.ErrRequestBodyTooLarge) {
			statusCode = http.StatusRequestEntityTooLarge
		}
		newAPIError = types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, statusCode, types.ErrOptionWithSkipRetry())
		return
	}
	imageRequest, ok := request.(*dto.ImageRequest)
	if !ok {
		newAPIError = types.NewErrorWithStatusCode(fmt.Errorf("invalid composite image request type %T", request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		return
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, relayFormat, imageRequest, nil)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeGenRelayInfoFailed)
		return
	}
	relayInfo.CompositeGroupName = policy.Name
	relayInfo.CompositeOperation = operation

	meta := imageRequest.GetTokenCountMeta()
	if setting.ShouldCheckPromptSensitive() {
		contains, words := service.CheckSensitiveText(meta.CombineText)
		if contains {
			logger.LogWarn(c, fmt.Sprintf("user sensitive words detected: %v", words))
			newAPIError = types.NewError(errors.New("sensitive words detected"), types.ErrorCodeSensitiveWordsDetected, types.ErrOptionWithSkipRetry())
			return
		}
	}
	tokens, err := service.EstimateRequestToken(c, meta, relayInfo)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeCountTokenFailed)
		return
	}
	relayInfo.SetEstimatePromptTokens(tokens)

	bodyStorage, err := common.GetBodyStorage(c)
	if err != nil {
		statusCode := http.StatusBadRequest
		if common.IsRequestBodyTooLargeError(err) || errors.Is(err, common.ErrRequestBodyTooLarge) {
			statusCode = http.StatusRequestEntityTooLarge
		}
		newAPIError = types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, statusCode, types.ErrOptionWithSkipRetry())
		return
	}

	defer func() {
		if newAPIError == nil {
			return
		}
		newAPIError = service.NormalizeViolationFeeError(newAPIError)
		if relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
		service.ChargeViolationFeeIfNeeded(c, relayInfo, newAPIError)
	}()

	routes := policy.RoutesFor(operation)
	for _, route := range routes {
		targetRequest, copyErr := common.DeepCopy(imageRequest)
		if copyErr != nil {
			newAPIError = types.NewError(copyErr, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
			return
		}
		targetRequest.Model = route.InternalModel
		relayInfo.Request = targetRequest
		relayInfo.BillingModelName = route.InternalModel
		relayInfo.CompositePhysicalGroup = route.PhysicalGroup
		relayInfo.CompositeRouteOrder = route.RouteOrder
		common.SetContextKey(c, constant.ContextKeyCompositePhysicalGroup, route.PhysicalGroup)
		common.SetContextKey(c, constant.ContextKeyCompositeBillingModel, route.InternalModel)
		common.SetContextKey(c, constant.ContextKeyCompositeRouteOrder, route.RouteOrder)
		relayInfo.TieredBillingSnapshot = nil
		relayInfo.QuotaClamp = nil
		relayInfo.UsingGroup = route.PhysicalGroup
		common.SetContextKey(c, constant.ContextKeyUsingGroup, route.PhysicalGroup)
		targetRequestInput, inputErr := helper.BuildBillingExprRequestInputFromRequest(targetRequest, relayInfo.RequestHeaders)
		if inputErr != nil {
			newAPIError = types.NewError(inputErr, types.ErrorCodeGenRelayInfoFailed)
			return
		}
		relayInfo.BillingRequestInput = &targetRequestInput

		priceData, priceErr := helper.ModelPriceHelper(c, relayInfo, tokens, targetRequest.GetTokenCountMeta())
		if priceErr != nil {
			newAPIError = types.NewErrorWithStatusCode(priceErr, types.ErrorCodeModelPriceError, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			return
		}
		if !priceData.FreeModel {
			if relayInfo.Billing == nil {
				newAPIError = service.PreConsumeBilling(c, priceData.QuotaToPreConsume, relayInfo)
				if newAPIError != nil {
					return
				}
			} else if reserveErr := relayInfo.Billing.Reserve(priceData.QuotaToPreConsume); reserveErr != nil {
				newAPIError = types.NewErrorWithStatusCode(reserveErr, types.ErrorCodePreConsumeTokenQuotaFailed, http.StatusForbidden, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
				return
			}
		}

		relayInfo.UsingGroup = policy.Name
		common.SetContextKey(c, constant.ContextKeyUsingGroup, policy.Name)
		for attempt := 0; attempt <= route.RetryCount; attempt++ {
			relayInfo.RetryIndex = attempt
			relayInfo.LastError = nil
			relayInfo.ChannelMeta = nil
			relayInfo.UpstreamModelName = ""
			relayInfo.IsModelMapped = false
			relayInfo.RequestConversionChain = nil
			relayInfo.FinalRequestRelayFormat = ""
			resetCompositeChannelContext(c)

			retry := attempt
			channel, _, selectErr := service.CacheGetRandomSatisfiedChannel(&service.RetryParam{
				Ctx:         c,
				TokenGroup:  route.PhysicalGroup,
				ModelName:   route.InternalModel,
				RequestPath: c.Request.URL.Path,
				Retry:       &retry,
			})
			if selectErr != nil || channel == nil {
				if selectErr == nil {
					selectErr = fmt.Errorf("no channel for group %s model %s", route.PhysicalGroup, route.InternalModel)
				}
				newAPIError = types.NewError(selectErr, types.ErrorCodeGetChannelFailed)
			} else {
				newAPIError = middleware.SetupContextForSelectedChannel(c, channel, route.InternalModel)
				if newAPIError == nil {
					addUsedChannel(c, channel.Id)
					if _, seekErr := bodyStorage.Seek(0, io.SeekStart); seekErr != nil {
						newAPIError = types.NewErrorWithStatusCode(seekErr, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
					} else {
						c.Request.Body = io.NopCloser(bodyStorage)
						newAPIError = relay.ImageHelper(c, relayInfo)
					}
				}
				if newAPIError != nil {
					processChannelError(c, *types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, common.GetContextKeyString(c, constant.ContextKeyChannelKey), channel.GetAutoBan()), newAPIError)
				}
			}

			if newAPIError == nil {
				relayInfo.LastError = nil
				return
			}
			relayInfo.LastError = newAPIError
			relayInfo.CompositeAttempts = append(relayInfo.CompositeAttempts, relaycommon.CompositeAttempt{
				RouteOrder:    route.RouteOrder,
				PhysicalGroup: route.PhysicalGroup,
				BillingModel:  route.InternalModel,
				ChannelId:     common.GetContextKeyInt(c, constant.ContextKeyChannelId),
				StatusCode:    newAPIError.StatusCode,
				ErrorCode:     string(newAPIError.GetErrorCode()),
			})
			if c.Writer.Written() || !shouldRetryCompositeImage(newAPIError, route.RetryStatusCodes) {
				return
			}
		}
	}

	if newAPIError == nil {
		newAPIError = types.NewError(errors.New("composite image group has no available route"), types.ErrorCodeGetChannelFailed)
	}
}

func shouldRetryCompositeImage(err *types.NewAPIError, statusExpression string) bool {
	if err == nil {
		return false
	}
	if types.IsChannelError(err) {
		return true
	}
	if types.IsSkipRetryError(err) {
		return false
	}
	if err.StatusCode < 100 || err.StatusCode > 599 {
		return true
	}
	return service.ShouldRetryCompositeStatus(statusExpression, err.StatusCode)
}

func resetCompositeChannelContext(c *gin.Context) {
	common.SetContextKey(c, constant.ContextKeyChannelId, 0)
	common.SetContextKey(c, constant.ContextKeyChannelName, "")
	common.SetContextKey(c, constant.ContextKeyChannelCreateTime, int64(0))
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "")
	common.SetContextKey(c, constant.ContextKeyChannelType, 0)
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{})
	common.SetContextKey(c, constant.ContextKeyChannelOtherSetting, dto.ChannelOtherSettings{})
	common.SetContextKey(c, constant.ContextKeyChannelOrganization, "")
	common.SetContextKey(c, constant.ContextKeyChannelAutoBan, false)
	common.SetContextKey(c, constant.ContextKeyChannelModelMapping, "")
	common.SetContextKey(c, constant.ContextKeyChannelStatusCodeMapping, "")
	common.SetContextKey(c, constant.ContextKeyChannelParamOverride, map[string]interface{}{})
	common.SetContextKey(c, constant.ContextKeyChannelHeaderOverride, map[string]interface{}{})
	common.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, false)
	common.SetContextKey(c, constant.ContextKeyChannelMultiKeyIndex, 0)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "")
	c.Set("api_version", "")
	c.Set("region", "")
	c.Set("plugin", "")
	c.Set("bot_id", "")
}
