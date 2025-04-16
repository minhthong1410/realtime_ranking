package middleware

import (
	"net/http"
)

type OnError func(err any, w http.ResponseWriter)

func RecoverWrap(h http.Handler, errorHandler OnError) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			r := recover()
			if r != nil {
				errorHandler(r, w)
			}
		}()
		h.ServeHTTP(w, r)
	})
}
