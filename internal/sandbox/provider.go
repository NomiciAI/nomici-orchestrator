package sandbox

import (
	"context"
	"fmt"
)

type Provider interface {
	Acquire(ctx context.Context, request CreateRecordRequest) (*Record, error)
	Get(ctx context.Context, runID string) (*Record, error)
	Release(ctx context.Context, runID string) error
}

type LocalProvider struct {
	store *Store
}

type ContainerProvider struct {
	store *Store
}

func NewLocalProvider(store *Store) *LocalProvider {
	return &LocalProvider{store: store}
}

func NewContainerProvider(store *Store) *ContainerProvider {
	return &ContainerProvider{store: store}
}

func (provider *LocalProvider) Acquire(ctx context.Context, request CreateRecordRequest) (*Record, error) {
	if provider == nil || provider.store == nil {
		return nil, fmt.Errorf("local sandbox provider is not initialized")
	}
	request.Intent.Mode = ModeLocal
	return provider.store.CreateForRun(ctx, request)
}

func (provider *LocalProvider) Get(ctx context.Context, runID string) (*Record, error) {
	if provider == nil || provider.store == nil {
		return nil, fmt.Errorf("local sandbox provider is not initialized")
	}
	return provider.store.GetByRun(ctx, runID)
}

func (provider *LocalProvider) Release(ctx context.Context, runID string) error {
	if provider == nil || provider.store == nil {
		return fmt.Errorf("local sandbox provider is not initialized")
	}
	return provider.store.ReleaseByRun(ctx, runID)
}

func (provider *ContainerProvider) Acquire(ctx context.Context, request CreateRecordRequest) (*Record, error) {
	if provider == nil || provider.store == nil {
		return nil, fmt.Errorf("container sandbox provider is not initialized")
	}
	request.Intent.Mode = ModeContainer
	return provider.store.CreateForRun(ctx, request)
}

func (provider *ContainerProvider) Get(ctx context.Context, runID string) (*Record, error) {
	if provider == nil || provider.store == nil {
		return nil, fmt.Errorf("container sandbox provider is not initialized")
	}
	return provider.store.GetByRun(ctx, runID)
}

func (provider *ContainerProvider) Release(ctx context.Context, runID string) error {
	if provider == nil || provider.store == nil {
		return fmt.Errorf("container sandbox provider is not initialized")
	}
	return provider.store.ReleaseByRun(ctx, runID)
}
