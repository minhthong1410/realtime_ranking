package httputil

type HttpResponse struct {
	Code int `json:"code"`
	//Total int64 `json:"total,omitempty"`
	Data any `json:"data"`
}
