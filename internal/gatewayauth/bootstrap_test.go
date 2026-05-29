package gatewayauth

import (
	"path/filepath"
	"testing"
	"time"
)

func TestBootstrapTokenIsConsumedOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), BootstrapFile)
	record, err := CreateBootstrap(path, time.Minute)
	if err != nil {
		t.Fatalf("create bootstrap token: %v", err)
	}
	ok, err := ConsumeBootstrap(path, record.Token, time.Now().UTC())
	if err != nil {
		t.Fatalf("consume bootstrap token: %v", err)
	}
	if !ok {
		t.Fatal("expected bootstrap token to be accepted")
	}
	ok, err = ConsumeBootstrap(path, record.Token, time.Now().UTC())
	if err != nil {
		t.Fatalf("consume bootstrap token again: %v", err)
	}
	if ok {
		t.Fatal("expected bootstrap token to be single-use")
	}
}

func TestExpiredBootstrapTokenIsRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), BootstrapFile)
	record, err := CreateBootstrap(path, -time.Minute)
	if err != nil {
		t.Fatalf("create bootstrap token: %v", err)
	}
	ok, err := ConsumeBootstrap(path, record.Token, time.Now().UTC())
	if err != nil {
		t.Fatalf("consume expired bootstrap token: %v", err)
	}
	if ok {
		t.Fatal("expected expired bootstrap token to be rejected")
	}
}
