package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
)

type successEnvelope struct {
	Data      any      `json:"data"`
	Warnings  []string `json:"warnings"`
	RequestID string   `json:"request_id"`
}

type errorEnvelope struct {
	Error     apiError `json:"error"`
	RequestID string   `json:"request_id"`
}

type apiError struct {
	Code        string         `json:"code"`
	Message     string         `json:"message"`
	Remediation string         `json:"remediation,omitempty"`
	Details     map[string]any `json:"details"`
}

func writeJSON(response http.ResponseWriter, status int, payload any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(payload)
}

func writeSuccess(response http.ResponseWriter, requestID string, data any, warnings []string) {
	if warnings == nil {
		warnings = []string{}
	}
	writeJSON(response, http.StatusOK, successEnvelope{
		Data:      data,
		Warnings:  warnings,
		RequestID: requestID,
	})
}

func writeError(response http.ResponseWriter, status int, requestID string, code string, message string, remediation string) {
	writeJSON(response, status, errorEnvelope{
		Error: apiError{
			Code:        code,
			Message:     message,
			Remediation: remediation,
			Details:     map[string]any{},
		},
		RequestID: requestID,
	})
}

func newRequestID() string {
	var bytes [4]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "req_unknown"
	}
	return "req_" + hex.EncodeToString(bytes[:])
}
