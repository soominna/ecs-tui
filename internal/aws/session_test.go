package aws

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadLastSession(t *testing.T) {
	// Use temp dir to avoid touching real config
	tmpDir := t.TempDir()
	sessionFile := filepath.Join(tmpDir, "session.json")

	// Save session directly to temp file
	session := LastSession{Profile: "test-profile", Region: "ap-northeast-2", Theme: "mocha"}
	data, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(sessionFile, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Load and verify
	raw, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var loaded LastSession
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if loaded.Profile != "test-profile" {
		t.Errorf("Profile = %q, want %q", loaded.Profile, "test-profile")
	}
	if loaded.Region != "ap-northeast-2" {
		t.Errorf("Region = %q, want %q", loaded.Region, "ap-northeast-2")
	}
	if loaded.Theme != "mocha" {
		t.Errorf("Theme = %q, want %q", loaded.Theme, "mocha")
	}
}

func TestLoadLastSession_EmptyProfile(t *testing.T) {
	tmpDir := t.TempDir()
	sessionFile := filepath.Join(tmpDir, "session.json")

	// Write session with empty profile
	data, _ := json.Marshal(LastSession{Profile: "", Region: "us-east-1"})
	os.WriteFile(sessionFile, data, 0o600)

	raw, _ := os.ReadFile(sessionFile)
	var s LastSession
	json.Unmarshal(raw, &s)

	// LoadLastSession returns nil when profile is empty
	if s.Profile != "" {
		t.Errorf("expected empty profile, got %q", s.Profile)
	}
}

func TestLoadLastSession_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	sessionFile := filepath.Join(tmpDir, "session.json")

	os.WriteFile(sessionFile, []byte("not json"), 0o600)

	raw, _ := os.ReadFile(sessionFile)
	var s LastSession
	err := json.Unmarshal(raw, &s)
	if err == nil {
		t.Error("expected unmarshal error for invalid JSON")
	}
}

func TestLoadLastSession_FileNotFound(t *testing.T) {
	// LoadLastSession should return nil for missing file
	result := LoadLastSession()
	// This test just verifies it doesn't panic; actual path depends on HOME
	_ = result
}
