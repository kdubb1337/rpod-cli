package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Resolved is the active configuration after flag + env + file resolution.
type Resolved struct {
	Profile string
	Account string
	APIKey  string
}

var current Resolved

// Resolve applies the precedence:
//
//	explicit --profile flag > --account flag > env > stored default profile
//
// Side effect: stores the result in package-level state for accessors below.
func Resolve(profile, account string) error {
	r := Resolved{Profile: profile, Account: account}

	store, err := load()
	if err != nil {
		return err
	}

	// Profile resolution: --profile flag > store.DefaultProfile.
	if r.Profile == "" {
		r.Profile = store.DefaultProfile
	}

	// Profile lookup is best-effort here. A missing profile is not fatal at
	// resolve time — `auth add` and `profile save` need to write profiles that
	// don't exist yet. Commands that actually need the resolved key (everything
	// hitting the API) will fail later with the structured `auth_missing` error.
	if r.Profile != "" {
		if p, ok := store.Profiles[r.Profile]; ok {
			if r.Account == "" {
				r.Account = p.Account
			}
			if r.APIKey == "" {
				r.APIKey = p.APIKey
			}
		}
	}

	// Env always overrides empty resolved values; flags already took priority.
	if r.Account == "" {
		r.Account = os.Getenv("RUNPOD_ACCOUNT")
	}
	if r.APIKey == "" {
		r.APIKey = os.Getenv("RUNPOD_API_KEY")
	}

	current = r
	return nil
}

// Current returns the resolved configuration.
func Current() Resolved { return current }

// --- on-disk store ---------------------------------------------------------

// Profile is a saved named configuration.
type Profile struct {
	Account string `json:"account,omitempty"`
	APIKey  string `json:"api_key,omitempty"`
}

// Store is the full ~/.runpod/config.json document.
type Store struct {
	DefaultProfile string             `json:"default_profile,omitempty"`
	Profiles       map[string]Profile `json:"profiles,omitempty"`
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".runpod"), nil
}

func configPath() (string, error) {
	d, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.json"), nil
}

func load() (*Store, error) {
	p, err := configPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return &Store{Profiles: map[string]Profile{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var s Store
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("config %s: %w", p, err)
	}
	if s.Profiles == nil {
		s.Profiles = map[string]Profile{}
	}
	return &s, nil
}

func save(s *Store) error {
	d, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d, 0o700); err != nil {
		return err
	}
	p, err := configPath()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}

// Load returns the on-disk store (creating an empty one if absent).
func Load() (*Store, error) { return load() }

// Save persists the store to disk with 0600 permissions.
func Save(s *Store) error { return save(s) }
