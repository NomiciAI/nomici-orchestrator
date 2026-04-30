package gateway

import (
	"encoding/json"
	"net/http"
	"time"
)

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version"`
	Time    string `json:"time"`
}

func healthHandler(options Options) http.HandlerFunc {
	version := options.Version
	if version == "" {
		version = "dev"
	}

	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(HealthResponse{
			Status:  "ok",
			Service: "nomici-gateway",
			Version: version,
			Time:    time.Now().UTC().Format(time.RFC3339),
		})
	}
}
