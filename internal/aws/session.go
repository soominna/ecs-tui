package aws

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// LastSession stores the most recently used profile and region.
type LastSession struct {
	Profile string `json:"profile"`
	Region  string `json:"region"`
}

func sessionFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "ecs-tui", "session.json")
}

// SaveLastSession persists the profile and region for next launch.
func SaveLastSession(profile, region string) error {
	p := sessionFilePath()
	if p == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(LastSession{Profile: profile, Region: region})
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
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
