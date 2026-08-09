package qianfan

type qianfanRequest struct {
	Model           string         `json:"model"`
	Type            string         `json:"type"`
	ModelParameters map[string]any `json:"model_parameters"`
}

type qianfanResponse struct {
	Code               int    `json:"code"`
	Message            string `json:"message"`
	FinalUnitDeduction any    `json:"final_unit_deduction"`
	Usage              struct {
		Credits any `json:"credits"`
	} `json:"usage"`
	Data struct {
		TaskID             string `json:"task_id"`
		SessionID          string `json:"session_id"`
		TaskStatus         string `json:"task_status"`
		TaskStatusMsg      string `json:"task_status_msg"`
		CreatedAt          int64  `json:"created_at"`
		UpdatedAt          int64  `json:"updated_at"`
		FinalUnitDeduction any    `json:"final_unit_deduction"`
		Usage              struct {
			Credits any `json:"credits"`
		} `json:"usage"`
		TaskResult struct {
			Videos []struct {
				Duration string `json:"duration"`
				ID       string `json:"id"`
				URL      string `json:"url"`
			} `json:"videos"`
		} `json:"task_result"`
	} `json:"data"`
}
