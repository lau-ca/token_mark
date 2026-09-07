package controller

import "github.com/QuantumNous/new-api/model"

func getGeminiVideoURL(_ *model.Channel, task *model.Task, _ string) (string, error) {
	return task.PrivateData.ResultURL, nil
}
func getVertexVideoURL(_ *model.Channel, task *model.Task) (string, error) {
	return task.PrivateData.ResultURL, nil
}
