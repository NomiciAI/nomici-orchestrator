package gateway

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/NomiciAI/nomici-orchestrator/internal/gatewayauth"
)

type bootstrapAuthRequest struct {
	Token string `json:"token"`
}

type bootstrapAuthResponse struct {
	GatewayToken string `json:"gateway_token"`
}

func bootstrapAuthHandler(options Options) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		var payload bootstrapAuthRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			writeError(response, http.StatusBadRequest, requestID, "invalid_request", "Request body must be JSON.", "")
			return
		}
		if payload.Token == "" {
			writeError(response, http.StatusBadRequest, requestID, "missing_bootstrap_token", "Bootstrap token is required.", "")
			return
		}
		ok, err := gatewayauth.ConsumeBootstrap(
			gatewayauth.BootstrapPathForDB(options.DBPath),
			payload.Token,
			time.Now().UTC(),
		)
		if err != nil {
			writeError(response, http.StatusInternalServerError, requestID, "bootstrap_failed", "Bootstrap token could not be checked.", "Run `nomici dev` again.")
			return
		}
		if !ok {
			writeError(response, http.StatusUnauthorized, requestID, "bootstrap_invalid", "Bootstrap token is invalid or expired.", "Run `nomici dev` or `nomici gateway open` again.")
			return
		}
		writeSuccess(response, requestID, bootstrapAuthResponse{GatewayToken: options.AuthToken}, nil)
	}
}

func localReconnectAuthHandler(options Options) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		requestID := newRequestID()
		if !requestLooksSameOrigin(request) {
			writeError(response, http.StatusForbidden, requestID, "local_reconnect_forbidden", "Local reconnect must be initiated from the Nomici Console.", "Run `nomici gateway open` from the workspace.")
			return
		}
		writeSuccess(response, requestID, bootstrapAuthResponse{GatewayToken: options.AuthToken}, nil)
	}
}

func requestLooksSameOrigin(request *http.Request) bool {
	if request.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	origin := request.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return parsed.Host == request.Host
}
