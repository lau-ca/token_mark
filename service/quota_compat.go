package service

import relaycommon "github.com/QuantumNous/new-api/relay/common"

type postConsumeQuotaResult struct {
	FundingApplied bool
	TokenApplied   bool
}

func postConsumeQuotaWithResult(info *relaycommon.RelayInfo, quota, pre int, send bool) (postConsumeQuotaResult, error) {
	err := PostConsumeQuota(info, quota, pre, send)
	return postConsumeQuotaResult{FundingApplied: err == nil, TokenApplied: err == nil}, err
}
