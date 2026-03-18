package config

import (
	"errors"
	"fmt"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultConfigPath = "watcher/internal/config/config.yaml"
	LegacyConfigPath  = "watcher/internal/config/event-engine.yaml"
	defaultStorePath  = "event-engine-badger"
	memory            = "memory"
)

var (
	ErrNoRules           = errors.New("config: no rules configured")
	ErrRuleMissingName   = errors.New("config: rule missing name")
	ErrRuleMissingAction = errors.New("config: rule missing actions")
	defaultConfigPaths   = func() []string { return []string{DefaultConfigPath, LegacyConfigPath} }
)

type WatchConfig struct {
	ResourceTracking ResourceTrackingConfig `yaml:"resource_tracking"`
	Rules            []Rule                 `yaml:"rules"`
	Actions          map[string]Action      `yaml:"actions"`
	Settings         Settings               `yaml:"settings"`
}

type ResourceTrackingConfig struct {
	Enabled   bool          `yaml:"enabled"`
	Storage   string        `yaml:"storage"`
	Path      string        `yaml:"path"`
	Fields    []string      `yaml:"fields"`
	Retention time.Duration `yaml:"retention"`
}

type Rule struct {
	Name       string        `yaml:"name"`
	Kind       string        `yaml:"kind"`
	Namespace  string        `yaml:"namespace"`
	Selector   Selector      `yaml:"selector"`
	Logic      string        `yaml:"logic"`
	For        time.Duration `yaml:"duration"`
	Expression string        `yaml:"expression,omitempty"`
	Condition  string        `yaml:"condition,omitempty"`
	Conditions []Condition   `yaml:"conditions"`
	Actions    []string      `yaml:"actions"`
}

type Selector struct {
	MatchLabels map[string]string `yaml:"matchLabels"`
}

type Condition struct {
	Field      string      `yaml:"field"`
	Operator   string      `yaml:"operator"`
	Value      interface{} `yaml:"value"`
	Expression string      `yaml:"expression"`
}

type Action struct {
	Type     string            `yaml:"type"`
	Template string            `yaml:"template"`
	Throttle Throttle          `yaml:"throttle"`
	Config   map[string]string `yaml:"config"`
}

type Throttle struct {
	MaxPerMinute int `yaml:"max_per_minute"`
	MaxPerHour   int `yaml:"max_per_hour"`
}

type Settings struct {
	CEL        CELSettings   `yaml:"cel"`
	QueueDepth int           `yaml:"queue_depth"`
	Metrics    MetricsConfig `yaml:"metrics"`
	Model      ModelSettings `yaml:"model"`
}

type CELSettings struct {
	Enabled bool `yaml:"enabled"`
}

type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Listen  string `yaml:"listen"`
}

type ModelSettings struct {
	Enabled       bool          `yaml:"enabled"`
	Endpoint      string        `yaml:"endpoint"`
	Timeout       time.Duration `yaml:"timeout"`
	APIKeyEnv     string        `yaml:"api_key_env"`
	SystemPrompt  string        `yaml:"system_prompt"`
	MinConfidence float64       `yaml:"min_confidence"`
}

func Load(path string) (*WatchConfig, error) {
	paths := resolvePaths(path)
	if len(paths) == 0 {
		return nil, fmt.Errorf("config: no configuration files found")
	}

	var merged *WatchConfig
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read config %s: %w", p, err)
		}

		var cfg WatchConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("unmarshal config %s: %w", p, err)
		}
		cfg.applyDefaults()

		if merged == nil {
			merged = &cfg
			continue
		}
		merged.merge(cfg)
	}

	if merged == nil {
		return nil, fmt.Errorf("config: failed to load files")
	}

	if err := merged.Validate(); err != nil {
		return nil, err
	}
	return merged, nil
}

func DefaultConfigPaths() []string {
	return defaultConfigPaths()
}

func ResolvedConfigPaths(path string) []string {
	return resolvePaths(path)
}

func resolveDefaultPath(rel string) string {
	if filepath.IsAbs(rel) {
		return rel
	}
	if p := tryPath(rel); p != "" {
		return p
	}
	if wd, err := os.Getwd(); err == nil {
		if p := searchUp(wd, rel); p != "" {
			return p
		}
	}
	if exe, err := os.Executable(); err == nil {
		if p := searchUp(filepath.Dir(exe), rel); p != "" {
			return p
		}
	}
	return ""
}

func searchUp(start, rel string) string {
	dir := start
	for {
		if p := tryPath(filepath.Join(dir, rel)); p != "" {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func tryPath(path string) string {
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

func resolvePaths(path string) []string {
	if path != "" {
		return []string{expandUser(path)}
	}

	var paths []string
	for _, rel := range DefaultConfigPaths() {
		if resolved := resolveDefaultPath(rel); resolved != "" {
			paths = append(paths, resolved)
		}
	}
	return paths
}

func expandUser(path string) string {
	if path == "" || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}

func (c *WatchConfig) Validate() error {
	if len(c.Rules) == 0 {
		return ErrNoRules
	}
	for _, r := range c.Rules {
		if r.Name == "" {
			return ErrRuleMissingName
		}
		if len(r.Actions) == 0 {
			return fmt.Errorf("%w: %s", ErrRuleMissingAction, r.Name)
		}
		if len(r.Conditions) == 0 && r.Expression == "" && r.Condition == "" {
			return fmt.Errorf("rule %s missing conditions or expression", r.Name)
		}
		for _, act := range r.Actions {
			if _, ok := c.Actions[act]; !ok {
				return fmt.Errorf("rule %s references unknown action %s", r.Name, act)
			}
		}
	}
	return nil
}

func (c *WatchConfig) applyDefaults() {
	if c.ResourceTracking.Storage == "" {
		if c.ResourceTracking.Enabled {
			c.ResourceTracking.Storage = "disk"
		} else {
			c.ResourceTracking.Storage = memory
		}
	}
	if c.ResourceTracking.Path == "" {
		c.ResourceTracking.Path = defaultStorePath
	}
	if c.ResourceTracking.Retention <= 0 {
		c.ResourceTracking.Retention = time.Hour
	}
	if c.Settings.QueueDepth == 0 {
		c.Settings.QueueDepth = 256
	}
	if c.Settings.Metrics.Listen == "" {
		c.Settings.Metrics.Listen = ":9095"
	}
	if c.Settings.Model.Timeout <= 0 {
		c.Settings.Model.Timeout = 5 * time.Second
	}
	if c.Settings.Model.MinConfidence <= 0 {
		c.Settings.Model.MinConfidence = 0.65
	}
	if c.Settings.Model.SystemPrompt == "" {
		c.Settings.Model.SystemPrompt = "You are a Kubernetes SRE anomaly detector. Return only strict JSON."
	}
	if c.Actions == nil {
		c.Actions = make(map[string]Action)
	}

	for i := range c.Rules {
		c.Rules[i].Kind = cases.Title(language.Und).String(strings.ToLower(c.Rules[i].Kind))
		if c.Rules[i].Logic == "" {
			c.Rules[i].Logic = "all"
		}
		if c.Rules[i].Expression == "" && c.Rules[i].Condition != "" {
			c.Rules[i].Expression = c.Rules[i].Condition
		}
	}
}

func (c *WatchConfig) merge(other WatchConfig) {
	c.ResourceTracking.Enabled = c.ResourceTracking.Enabled || other.ResourceTracking.Enabled
	if other.ResourceTracking.Storage != "" {
		c.ResourceTracking.Storage = other.ResourceTracking.Storage
	}
	if other.ResourceTracking.Path != "" && other.ResourceTracking.Path != defaultStorePath {
		c.ResourceTracking.Path = other.ResourceTracking.Path
	}
	if other.ResourceTracking.Retention > 0 {
		c.ResourceTracking.Retention = other.ResourceTracking.Retention
	}
	c.ResourceTracking.Fields = mergeStrings(c.ResourceTracking.Fields, other.ResourceTracking.Fields)

	c.Rules = append(c.Rules, other.Rules...)

	if c.Actions == nil {
		c.Actions = make(map[string]Action)
	}
	for k, v := range other.Actions {
		c.Actions[k] = v
	}

	if other.Settings.QueueDepth > 0 {
		c.Settings.QueueDepth = other.Settings.QueueDepth
	}
	if other.Settings.CEL.Enabled {
		c.Settings.CEL.Enabled = true
	}
	if other.Settings.Model.Enabled {
		c.Settings.Model.Enabled = true
	}
	if other.Settings.Model.Endpoint != "" {
		c.Settings.Model.Endpoint = other.Settings.Model.Endpoint
	}
	if other.Settings.Model.Timeout > 0 {
		c.Settings.Model.Timeout = other.Settings.Model.Timeout
	}
	if other.Settings.Model.APIKeyEnv != "" {
		c.Settings.Model.APIKeyEnv = other.Settings.Model.APIKeyEnv
	}
	if other.Settings.Model.SystemPrompt != "" {
		c.Settings.Model.SystemPrompt = other.Settings.Model.SystemPrompt
	}
	if other.Settings.Model.MinConfidence > 0 {
		c.Settings.Model.MinConfidence = other.Settings.Model.MinConfidence
	}
}

func mergeStrings(base, extra []string) []string {
	if len(extra) == 0 {
		return base
	}
	seen := make(map[string]struct{}, len(base)+len(extra))
	var out []string

	add := func(list []string) {
		for _, v := range list {
			trimmed := strings.TrimSpace(v)
			key := strings.ToLower(trimmed)
			if key == "" {
				continue
			}
			if _, ok := seen[key]; !ok {
				seen[key] = struct{}{}
				out = append(out, trimmed)
			}
		}
	}

	add(base)
	add(extra)
	return out
}
