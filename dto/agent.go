package dto

type AgentGroupMarginRequest struct {
	Group                 string   `json:"group"`
	GrossMarginRate       float64  `json:"gross_margin_rate"`
	PlatformRetentionRate *float64 `json:"platform_retention_rate"`
}

type AgentConfigRequest struct {
	Enabled      *bool                     `json:"enabled"`
	Remark       *string                   `json:"remark"`
	GroupMargins []AgentGroupMarginRequest `json:"group_margins"`
}

type AgentSettlementRequest struct {
	Cutoff           *int64  `json:"cutoff"`
	PaymentReference *string `json:"payment_reference"`
}
