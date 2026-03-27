package httpserver

import (
	"encoding/json"
	"net/http"
)

type Problem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func WriteProblem(w http.ResponseWriter, status int, code, msg string) {
	WriteJSON(w, status, Problem{Code: code, Message: msg})
}
