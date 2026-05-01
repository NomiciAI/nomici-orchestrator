package sharedcontext

import "errors"

var (
	ErrNilItem          = errors.New("context item is nil")
	ErrNilSnapshot      = errors.New("context snapshot is nil")
	ErrMissingProjectID = errors.New("project_id is required")
	ErrMissingRunID     = errors.New("run_id is required")
)
