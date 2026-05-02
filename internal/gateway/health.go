package gateway

import (
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"time"
)

type HealthResponse struct {
	Status  string        `json:"status"`
	Service string        `json:"service"`
	Version string        `json:"version"`
	Time    string        `json:"time"`
	Process ProcessHealth `json:"process,omitempty"`
}

type ProcessHealth struct {
	PID        int `json:"pid"`
	OpenFDs    int `json:"open_fds,omitempty"`
	Goroutines int `json:"goroutines,omitempty"`
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
			Process: processHealth(),
		})
	}
}

func processHealth() ProcessHealth {
	return ProcessHealth{
		PID:        os.Getpid(),
		OpenFDs:    openFDCount(),
		Goroutines: runtime.NumGoroutine(),
	}
}
