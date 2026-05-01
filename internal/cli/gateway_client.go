package cli

import (
	"net/http"

	"github.com/NomiciAI/nomici-orchestrator/internal/gatewayauth"
)

func addGatewayAuth(request *http.Request, dbPath string) error {
	token, err := gatewayauth.LoadForClient(dbPath)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	return nil
}
