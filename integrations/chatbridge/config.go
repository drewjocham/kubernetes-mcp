package chatbridge

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server        ServerConfig              `yaml:"server"`
	GoogleChat    GoogleChatConfig          `yaml:"google_chat"`
	Investigation InvestigationConfig       `yaml:"investigation"`
	Providers     map[string]ProviderConfig `yaml:"providers"`
	Trigger       TriggerConfig             `yaml:"trigger"`
	Reliability   ReliabilityConfig         `yaml:"reliability"`
	Reporting     ReportingConfig           `yaml:"reporting"`
}

type ServerConfig struct {
	Listen string `yaml:"listen"`
}

type GoogleChatConfig struct {
	AuthHeader   string `yaml:"auth_header"`
	AuthTokenEnv string `yaml:"auth_token_env"`
}

type InvestigationConfig struct {
	Provider           string        `yaml:"provider"`
	Timeout            time.Duration `yaml:"timeout"`
	PollInterval       time.Duration `yaml:"poll_interval"`
	PromptTemplate     string        `yaml:"prompt_template"`
	FallbackActionPlan []string      `yaml:"fallback_action_plan"`
}

type ProviderConfig struct {
	BaseURL            string            `yaml:"base_url"`
	APIKeyEnv          string            `yaml:"api_key_env"`
	StartMethod        string            `yaml:"start_method"`
	StartPath          string            `yaml:"start_path"`
	StartBodyTemplate  string            `yaml:"start_body_template"`
	StatusMethod       string            `yaml:"status_method"`
	StatusPathTemplate string            `yaml:"status_path_template"`
	Headers            map[string]string `yaml:"headers"`
	RunIDPaths         []string          `yaml:"run_id_paths"`
	StatePaths         []string          `yaml:"state_paths"`
	OutputPaths        []string          `yaml:"output_paths"`
	ErrorPaths         []string          `yaml:"error_paths"`
	TerminalStates     []string          `yaml:"terminal_states"`
	SuccessStates      []string          `yaml:"success_states"`
}

type TriggerConfig struct {
	RequirePrefixes    []string `yaml:"require_prefixes"`
	KubernetesKeywords []string `yaml:"kubernetes_keywords"`
	SolaceKeywords     []string `yaml:"solace_keywords"`
	AllowSpaces        []string `yaml:"allow_spaces"`
}

type ReliabilityConfig struct {
	IdempotencyTTL time.Duration `yaml:"idempotency_ttl"`
	RetryCount     int           `yaml:"retry_count"`
	RetryBackoff   time.Duration `yaml:"retry_backoff"`
}

type ReportingConfig struct {
	WebhookURL    string `yaml:"webhook_url"`
	WebhookURLEnv string `yaml:"webhook_url_env"`
}

func LoadConfig(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse yaml: %w", err)
	}
	applyDefaults(&cfg)
	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Server.Listen == "" {
		cfg.Server.Listen = ":8092"
	}
	if cfg.GoogleChat.AuthHeader == "" {
		cfg.GoogleChat.AuthHeader = "X-Bridge-Token"
	}
	if cfg.Investigation.Timeout <= 0 {
		cfg.Investigation.Timeout = 5 * time.Minute
	}
	if cfg.Investigation.PollInterval <= 0 {
		cfg.Investigation.PollInterval = 5 * time.Second
	}
	if cfg.Reliability.IdempotencyTTL <= 0 {
		cfg.Reliability.IdempotencyTTL = 10 * time.Minute
	}
	if cfg.Reliability.RetryCount < 0 {
		cfg.Reliability.RetryCount = 0
	}
	if cfg.Reliability.RetryBackoff <= 0 {
		cfg.Reliability.RetryBackoff = 2 * time.Second
	}
}
