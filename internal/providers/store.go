package providers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (store *Store) Save(ctx context.Context, profile *Profile) error {
	if err := profile.Validate(); err != nil {
		return err
	}

	capabilities := profile.Capabilities
	if capabilities == nil {
		capabilities = map[string]string{}
	}
	capabilitiesJSON, err := json.Marshal(capabilities)
	if err != nil {
		return fmt.Errorf("marshal provider capabilities: %w", err)
	}

	costJSON, err := json.Marshal(map[string]float64{
		"input":  profile.CostPer1MInput,
		"output": profile.CostPer1MOutput,
	})
	if err != nil {
		return fmt.Errorf("marshal provider cost: %w", err)
	}

	_, err = store.db.ExecContext(ctx, `
INSERT INTO provider_profiles (
	id, name, kind, base_url, model, api_key_env, capabilities_json,
	context_window, cost_json, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
ON CONFLICT(id) DO UPDATE SET
	name = excluded.name,
	kind = excluded.kind,
	base_url = excluded.base_url,
	model = excluded.model,
	api_key_env = excluded.api_key_env,
	capabilities_json = excluded.capabilities_json,
	context_window = excluded.context_window,
	cost_json = excluded.cost_json,
	updated_at = datetime('now')`,
		profile.ID,
		profile.Name,
		NormalizeKind(profile.Kind),
		profile.BaseURL,
		profile.Model,
		profile.APIKeyEnv,
		string(capabilitiesJSON),
		profile.ContextWindow,
		string(costJSON),
	)
	if err != nil {
		return fmt.Errorf("save provider profile: %w", err)
	}

	return nil
}

func (store *Store) Get(ctx context.Context, id string) (*Profile, error) {
	row := store.db.QueryRowContext(ctx, `
SELECT id, name, kind, base_url, model, api_key_env, capabilities_json,
	context_window, cost_json, created_at, updated_at
FROM provider_profiles
WHERE id = ?`, id)

	profile, err := scanProfile(row)
	if err != nil {
		return nil, err
	}
	return profile, nil
}

func (store *Store) List(ctx context.Context) ([]*Profile, error) {
	rows, err := store.db.QueryContext(ctx, `
SELECT id, name, kind, base_url, model, api_key_env, capabilities_json,
	context_window, cost_json, created_at, updated_at
FROM provider_profiles
ORDER BY name, id`)
	if err != nil {
		return nil, fmt.Errorf("list provider profiles: %w", err)
	}
	defer rows.Close()

	var profiles []*Profile
	for rows.Next() {
		profile, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list provider profiles: %w", err)
	}

	return profiles, nil
}

func (store *Store) Delete(ctx context.Context, id string) error {
	if _, err := store.db.ExecContext(ctx, "DELETE FROM provider_profiles WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete provider profile: %w", err)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProfile(row rowScanner) (*Profile, error) {
	var profile Profile
	var capabilitiesJSON string
	var costJSON string
	if err := row.Scan(
		&profile.ID,
		&profile.Name,
		&profile.Kind,
		&profile.BaseURL,
		&profile.Model,
		&profile.APIKeyEnv,
		&capabilitiesJSON,
		&profile.ContextWindow,
		&costJSON,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("scan provider profile: %w", err)
	}

	if capabilitiesJSON == "" {
		capabilitiesJSON = "{}"
	}
	if err := json.Unmarshal([]byte(capabilitiesJSON), &profile.Capabilities); err != nil {
		return nil, fmt.Errorf("decode provider capabilities: %w", err)
	}
	if profile.Capabilities == nil {
		profile.Capabilities = map[string]string{}
	}

	var cost map[string]float64
	if costJSON != "" {
		if err := json.Unmarshal([]byte(costJSON), &cost); err != nil {
			return nil, fmt.Errorf("decode provider cost: %w", err)
		}
	}
	profile.CostPer1MInput = cost["input"]
	profile.CostPer1MOutput = cost["output"]

	return &profile, nil
}
