package gateway

import (
	"net/http"
	"strings"

	"github.com/NomiciAI/nomici-orchestrator/internal/gatewayauth"
)

func bearerAuthMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			requestID := newRequestID()
			provided := bearerToken(request.Header.Get("Authorization"))
			if provided == "" {
				provided = request.Header.Get("X-Nomici-Gateway-Token")
			}
			if !gatewayauth.Matches(token, provided) {
				writeError(response, http.StatusUnauthorized, requestID, "unauthorized", "Gateway token is required.", "Run `nomici gateway token show` in the same workspace that started the Gateway, or set NOMICI_GATEWAY_TOKEN.")
				return
			}
			next.ServeHTTP(response, request)
		})
	}
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}
