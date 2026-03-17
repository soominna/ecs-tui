package aws

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LastSession stores the most recently used profile, region, and theme.
type LastSession struct {
	Profile string `json:"profile"`
	Region  string `json:"region"`
	Theme   string `json:"theme,omitempty"`
}

func sessionFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "ecs-tui", "session.json")
}

// SaveLastSession persists the profile, region, and theme for next launch.
func SaveLastSession(profile, region, theme string) error {
	p := sessionFilePath()
	if p == "" {
		return nil
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating session dir: %w", err)
	}
	// Enforce directory permissions even if it already existed
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("setting session dir permissions: %w", err)
	}
	data, err := json.Marshal(LastSession{Profile: profile, Region: region, Theme: theme})
	if err != nil {
		return fmt.Errorf("marshaling session: %w", err)
	}
	// Atomic write: write to temp file, set permissions, then rename
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("writing session file: %w", err)
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("setting session file permissions: %w", err)
	}
	if err := os.Rename(tmp, p); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("renaming session file: %w", err)
	}
	return nil
}

// LoadLastSession returns the previously saved session, or nil if none exists.
func LoadLastSession() *LastSession {
	p := sessionFilePath()
	if p == "" {
		return nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var s LastSession
	if err := json.Unmarshal(data, &s); err != nil {
		return nil
	}
	if s.Profile == "" {
		return nil
	}
	return &s
}
