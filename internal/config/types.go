package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const DefaultConfigPath = "~/.kube-watcher/event-engine.yaml"
const defaultStorePath = "~/.kube-watcher/event-engine-badger"

var (
	ErrNoRules           = errors.New("config: no rules configured")
	ErrRuleMissingName   = errors.New("config: rule missing name")
	ErrRuleMissingAction = errors.New("config: rule missing actions")
)

type WatchConfig struct {
	ResourceTracking ResourceTrackingConfig `yaml:"resource_tracking"`
	Rules            []Rule                 `yaml:"rules"`
	Actions          map[string]Action      `yaml:"actions"`
	Settings         Settings               `yaml:"settings"`
}

type ResourceTrackingConfig struct {
	Enabled bool     `yaml:"enabled"`
	Storage string   `yaml:"storage"`
	Path    string   `yaml:"path"`
	Fields  []string `yaml:"fields"`
}

type Rule struct {
	Name       string        `yaml:"name"`
	Kind       string        `yaml:"kind"`
	Namespace  string        `yaml:"namespace"`
	Selector   Selector      `yaml:"selector"`
	Logic      string        `yaml:"logic"`
	For        time.Duration `yaml:"duration"`
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
	CEL struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"cel"`
	QueueDepth int `yaml:"queue_depth"`
}

func Load(path string) (*WatchConfig, error) {
	if path == "" {
		path = DefaultConfigPath
	}
	if path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(home, path[1:])
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg WatchConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
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
		c.ResourceTracking.Storage = "memory"
	}
	if c.ResourceTracking.Path == "" {
		c.ResourceTracking.Path = defaultStorePath
	}
	if c.Settings.QueueDepth == 0 {
		c.Settings.QueueDepth = 256
	}
	if c.Actions == nil {
		c.Actions = make(map[string]Action)
	}
	// normalize kinds
	for i := range c.Rules {
		c.Rules[i].Kind = strings.Title(strings.ToLower(c.Rules[i].Kind))
		if c.Rules[i].Logic == "" {
			c.Rules[i].Logic = "all"
		}
	}
}
