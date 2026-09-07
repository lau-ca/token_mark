package controller

import (
	"errors"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"net/http"
)

func executeTaskSubmission(c *gin.Context, info *relaycommon.RelayInfo) (*taskSubmissionOutcome, *dto.TaskError) {
	result, taskErr := relay.RelayTaskSubmit(c, info)
	if taskErr != nil {
		return nil, taskErr
	}
	if result == nil {
		return nil, service.TaskErrorWrapperLocal(errors.New("task submission returned no result"), "task_submit_failed", http.StatusBadGateway)
	}
	return &taskSubmissionOutcome{Result: result, RelayInfo: info}, nil
}
