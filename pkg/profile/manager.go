package profile

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	ErrProfileNameRequired  = errors.New("profile: profile name is required")
	ErrProfileAlreadyExists = errors.New("profile: profile already exists")
	ErrProfileNotFound      = errors.New("profile: profile not found")
	ErrUnknownProvider      = errors.New("profile: unknown secret provider")
)

type SecretReference struct {
	Provider string `yaml:"provider" json:"provider"`
	Ref      string `yaml:"ref" json:"ref"`
}

type Profile struct {
	Name        string                     `yaml:"name" json:"name"`
	Description string                     `yaml:"description,omitempty" json:"description,omitempty"`
	Config      map[string]string          `yaml:"config,omitempty" json:"config,omitempty"`
	Env         map[string]string          `yaml:"env,omitempty" json:"env,omitempty"`
	Secrets     map[string]SecretReference `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	CreatedAt   string                     `yaml:"created_at,omitempty" json:"created_at,omitempty"`
	UpdatedAt   string                     `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
}

type Config struct {
	Active   string              `yaml:"active,omitempty" json:"active,omitempty"`
	Profiles map[string]*Profile `yaml:"profiles,omitempty" json:"profiles,omitempty"`
}

type SecretResolver interface {
	Resolve(provider, ref string) (string, error)
}

type Manager struct {
	path     string
	resolver SecretResolver
}

func NewManager(path string) *Manager {
	return &Manager{
		path:     path,
		resolver: DefaultSecretResolver{},
	}
}

func (m *Manager) WithResolver(resolver SecretResolver) *Manager {
	if resolver != nil {
		m.resolver = resolver
	}
	return m
}

func (m *Manager) Path() string {
	return m.path
}

func (m *Manager) Load() (*Config, error) {
	cfg := &Config{
		Profiles: map[string]*Profile{},
	}

	data, err := os.ReadFile(m.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read profile config: %w", err)
	}
	if len(data) == 0 {
		return cfg, nil
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("decode profile config: %w", err)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]*Profile{}
	}
	return cfg, nil
}

func (m *Manager) Save(cfg *Config) error {
	if cfg == nil {
		cfg = &Config{Profiles: map[string]*Profile{}}
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]*Profile{}
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode profile config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return fmt.Errorf("create profile directory: %w", err)
	}
	if err := os.WriteFile(m.path, data, 0o600); err != nil {
		return fmt.Errorf("write profile config: %w", err)
	}
	return nil
}

func (m *Manager) Create(name, description string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrProfileNameRequired
	}
	cfg, err := m.Load()
	if err != nil {
		return err
	}
	if _, ok := cfg.Profiles[name]; ok {
		return ErrProfileAlreadyExists
	}
	now := time.Now().UTC().Format(time.RFC3339)
	cfg.Profiles[name] = &Profile{
		Name:        name,
		Description: description,
		Config:      map[string]string{},
		Env:         map[string]string{},
		Secrets:     map[string]SecretReference{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if cfg.Active == "" {
		cfg.Active = name
	}
	return m.Save(cfg)
}

func (m *Manager) SetActive(name string) error {
	cfg, err := m.Load()
	if err != nil {
		return err
	}
	if _, ok := cfg.Profiles[name]; !ok {
		return ErrProfileNotFound
	}
	cfg.Active = name
	return m.Save(cfg)
}

func (m *Manager) SetConfigOverride(name, key, value string) error {
	cfg, err := m.Load()
	if err != nil {
		return err
	}
	prof, ok := cfg.Profiles[name]
	if !ok {
		return ErrProfileNotFound
	}
	if prof.Config == nil {
		prof.Config = map[string]string{}
	}
	prof.Config[key] = value
	prof.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return m.Save(cfg)
}

func (m *Manager) SetEnvValue(name, key, value string) error {
	cfg, err := m.Load()
	if err != nil {
		return err
	}
	prof, ok := cfg.Profiles[name]
	if !ok {
		return ErrProfileNotFound
	}
	if prof.Env == nil {
		prof.Env = map[string]string{}
	}
	prof.Env[key] = value
	prof.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return m.Save(cfg)
}

func (m *Manager) SetSecretRef(name, key, provider, ref string) error {
	cfg, err := m.Load()
	if err != nil {
		return err
	}
	prof, ok := cfg.Profiles[name]
	if !ok {
		return ErrProfileNotFound
	}
	if prof.Secrets == nil {
		prof.Secrets = map[string]SecretReference{}
	}
	prof.Secrets[key] = SecretReference{
		Provider: provider,
		Ref:      ref,
	}
	prof.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return m.Save(cfg)
}

func (m *Manager) ActiveConfigOverrides() (map[string]string, error) {
	cfg, err := m.Load()
	if err != nil {
		return nil, err
	}
	if cfg.Active == "" {
		return map[string]string{}, nil
	}
	prof, ok := cfg.Profiles[cfg.Active]
	if !ok || prof == nil || prof.Config == nil {
		return map[string]string{}, nil
	}
	out := make(map[string]string, len(prof.Config))
	for k, v := range prof.Config {
		out[k] = v
	}
	return out, nil
}

func (m *Manager) ResolveEnv(name string) (map[string]string, error) {
	cfg, err := m.Load()
	if err != nil {
		return nil, err
	}
	if name == "" {
		name = cfg.Active
	}
	prof, ok := cfg.Profiles[name]
	if !ok || prof == nil {
		return nil, ErrProfileNotFound
	}

	resolved := map[string]string{}
	for k, v := range prof.Env {
		resolved[k] = v
	}
	for envName, secret := range prof.Secrets {
		value, err := m.resolver.Resolve(secret.Provider, secret.Ref)
		if err != nil {
			return nil, fmt.Errorf("resolve secret %q: %w", envName, err)
		}
		resolved[envName] = value
	}
	return resolved, nil
}

func MaskedEnv(env map[string]string) map[string]string {
	out := make(map[string]string, len(env))
	for k, v := range env {
		if strings.Contains(strings.ToLower(k), "token") ||
			strings.Contains(strings.ToLower(k), "secret") ||
			strings.Contains(strings.ToLower(k), "password") ||
			strings.Contains(strings.ToLower(k), "key") {
			if v == "" {
				out[k] = ""
			} else {
				out[k] = "******"
			}
			continue
		}
		out[k] = v
	}
	return out
}

type DefaultSecretResolver struct{}

func (DefaultSecretResolver) Resolve(provider, ref string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "env":
		val := os.Getenv(ref)
		if val == "" {
			return "", fmt.Errorf("environment variable %q is empty or unset", ref)
		}
		return val, nil
	case "keychain":
		service, account := parseKeychainRef(ref)
		cmd := exec.Command("security", "find-generic-password", "-s", service, "-a", account, "-w")
		out, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("read keychain secret (%s/%s): %w", service, account, err)
		}
		return strings.TrimSpace(string(out)), nil
	default:
		return "", ErrUnknownProvider
	}
}

func parseKeychainRef(ref string) (service string, account string) {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	trimmed := strings.TrimSpace(ref)
	return trimmed, trimmed
}
