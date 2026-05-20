package uploads

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/NomiciAI/nomici-orchestrator/internal/store"
)

func TestStoreCreatesAndListsUploads(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	uploadStore := NewStore(db)
	ctx := context.Background()

	upload, err := uploadStore.Create(ctx, CreateRequest{
		SessionID:   "session_1",
		RunID:       "run_1",
		Filename:    "input.txt",
		Path:        "/tmp/input.txt",
		SizeBytes:   12,
		ContentType: "text/plain",
	})
	if err != nil {
		t.Fatal(err)
	}
	if upload.UploadID == "" || upload.Status != StatusReady {
		t.Fatalf("unexpected upload: %+v", upload)
	}
	uploads, err := uploadStore.List(ctx, "session_1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(uploads) != 1 || uploads[0].UploadID != upload.UploadID {
		t.Fatalf("expected listed upload, got %+v", uploads)
	}
}
