package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const (
	DefaultMaxDepth                = 5
	MaxScanDepth                   = 12
	DefaultRefreshIntervalSeconds  = 60
	MinimumRefreshIntervalSeconds  = 15
	MaximumRefreshIntervalSeconds  = 3600
	DefaultDiffDisplayByteLimit    = 900000
	DefaultGitCommandTimeoutMillis = 30000
)

type Settings struct {
	LastRoot               string `json:"lastRoot"`
	MaxDepth               int    `json:"maxDepth"`
	AutoRefresh            bool   `json:"autoRefresh"`
	RefreshIntervalSeconds int    `json:"refreshIntervalSeconds"`
	AutoFetch              bool   `json:"autoFetch"`
	AutoPullCleanRepos     bool   `json:"autoPullCleanRepos"`
	OnlyPullCleanRepos     bool   `json:"onlyPullCleanRepos"`
	IdeaPath               string `json:"ideaPath"`
	DiffDisplayByteLimit   int    `json:"diffDisplayByteLimit"`
}

func DefaultSettings() Settings {
	return Settings{
		MaxDepth:               DefaultMaxDepth,
		RefreshIntervalSeconds: DefaultRefreshIntervalSeconds,
		OnlyPullCleanRepos:     true,
		DiffDisplayByteLimit:   DefaultDiffDisplayByteLimit,
	}
}

func (s Settings) Normalize() Settings {
	if s.MaxDepth <= 0 {
		s.MaxDepth = DefaultMaxDepth
	}
	if s.MaxDepth > MaxScanDepth {
		s.MaxDepth = MaxScanDepth
	}
	if s.RefreshIntervalSeconds < MinimumRefreshIntervalSeconds {
		s.RefreshIntervalSeconds = MinimumRefreshIntervalSeconds
	}
	if s.RefreshIntervalSeconds > MaximumRefreshIntervalSeconds {
		s.RefreshIntervalSeconds = MaximumRefreshIntervalSeconds
	}
	if s.DiffDisplayByteLimit <= 0 {
		s.DiffDisplayByteLimit = DefaultDiffDisplayByteLimit
	}
	return s
}

type SettingsStore struct {
	appName  string
	fileName string
}

func NewSettingsStore(appName string, fileName string) *SettingsStore {
	return &SettingsStore{appName: appName, fileName: fileName}
}

func (s *SettingsStore) Load() (Settings, error) {
	path, err := s.path()
	if err != nil {
		return Settings{}, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultSettings(), nil
		}
		return Settings{}, err
	}
	settings := DefaultSettings()
	if err := json.Unmarshal(content, &settings); err != nil {
		return Settings{}, err
	}
	return settings.Normalize(), nil
}

func (s *SettingsStore) Save(settings Settings) error {
	path, err := s.path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(settings.Normalize(), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func (s *SettingsStore) path() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, s.appName, s.fileName), nil
}
