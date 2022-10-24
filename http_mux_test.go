package snack

import (
	"net/http"
	"testing"
)

func TestServeMux_Handle(t *testing.T) {
	mux := new(ServeMux)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})

	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello snack ..."))
	})

	_ = http.ListenAndServe(":8081", mux)
}
