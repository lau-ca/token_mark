package common

func ResolveTaskDuration(req TaskSubmitReq) (int, error) {
	if req.Duration > 0 {
		return req.Duration, nil
	}
	return 0, nil
}
