package httputil

import (
	"encoding/json"
	"net/http"
)

func RenderJSON(httpCode int, w http.ResponseWriter, v interface{}) error {
	js, err := json.Marshal(v)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)
	w.Write(js)
	return nil
}
