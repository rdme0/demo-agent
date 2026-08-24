package dto

type CallbackResponse struct {
	IsSuccess bool            `json:"isSuccess"`
	Result    *CallbackResult `json:"result"`
}

type CallbackResult struct {
	Output any `json:"output"`
}
