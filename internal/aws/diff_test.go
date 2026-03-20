package aws

import "testing"

func TestDiffTaskDefinitions_NilInputs(t *testing.T) {
	if diffs := DiffTaskDefinitions(nil, nil); diffs != nil {
		t.Errorf("expected nil, got %v", diffs)
	}
	if diffs := DiffTaskDefinitions(&TaskDefinitionDetail{}, nil); diffs != nil {
		t.Errorf("expected nil, got %v", diffs)
	}
}

func TestDiffTaskDefinitions_NoDiff(t *testing.T) {
	td := &TaskDefinitionDetail{
		Family:   "app",
		Revision: 5,
		CPU:      "256",
		Memory:   "512",
		Images:   map[string]string{"web": "nginx:1.24"},
		Environment: map[string]map[string]string{
			"web": {"LOG_LEVEL": "info"},
		},
	}
	diffs := DiffTaskDefinitions(td, td)
	if len(diffs) != 0 {
		t.Errorf("expected no diffs, got %d: %v", len(diffs), diffs)
	}
}

func TestDiffTaskDefinitions_Changed(t *testing.T) {
	old := &TaskDefinitionDetail{
		CPU:    "256",
		Memory: "512",
		Images: map[string]string{"web": "nginx:1.24"},
		Environment: map[string]map[string]string{
			"web": {"LOG_LEVEL": "debug"},
		},
	}
	new := &TaskDefinitionDetail{
		CPU:    "512",
		Memory: "1024",
		Images: map[string]string{"web": "nginx:1.25"},
		Environment: map[string]map[string]string{
			"web": {"LOG_LEVEL": "info"},
		},
	}
	diffs := DiffTaskDefinitions(old, new)

	expected := map[string]string{
		"CPU":                "changed",
		"Memory":             "changed",
		"Image (web)":        "changed",
		"Env.LOG_LEVEL (web)": "changed",
	}
	if len(diffs) != len(expected) {
		t.Fatalf("expected %d diffs, got %d: %v", len(expected), len(diffs), diffs)
	}
	for _, d := range diffs {
		kind, ok := expected[d.Field]
		if !ok {
			t.Errorf("unexpected diff field: %s", d.Field)
			continue
		}
		if d.Kind != kind {
			t.Errorf("field %s: expected kind %q, got %q", d.Field, kind, d.Kind)
		}
	}
}

func TestDiffTaskDefinitions_AddedRemoved(t *testing.T) {
	old := &TaskDefinitionDetail{
		Images: map[string]string{"web": "nginx:1.24"},
		Environment: map[string]map[string]string{
			"web": {"OLD_KEY": "val"},
		},
	}
	new := &TaskDefinitionDetail{
		Images: map[string]string{"web": "nginx:1.24", "sidecar": "envoy:1.0"},
		Environment: map[string]map[string]string{
			"web": {"NEW_KEY": "val2"},
		},
	}
	diffs := DiffTaskDefinitions(old, new)

	kinds := make(map[string]string)
	for _, d := range diffs {
		kinds[d.Field] = d.Kind
	}

	if kinds["Image (sidecar)"] != "added" {
		t.Errorf("expected sidecar image added, got %q", kinds["Image (sidecar)"])
	}
	if kinds["Env.OLD_KEY (web)"] != "removed" {
		t.Errorf("expected OLD_KEY removed, got %q", kinds["Env.OLD_KEY (web)"])
	}
	if kinds["Env.NEW_KEY (web)"] != "added" {
		t.Errorf("expected NEW_KEY added, got %q", kinds["Env.NEW_KEY (web)"])
	}
}

func TestDiffTaskDefinitions_RoleChanges(t *testing.T) {
	old := &TaskDefinitionDetail{
		TaskRoleARN: "arn:old-role",
		ExecRoleARN: "arn:exec-role",
		Images:      map[string]string{},
		Environment: map[string]map[string]string{},
	}
	new := &TaskDefinitionDetail{
		TaskRoleARN: "arn:new-role",
		ExecRoleARN: "arn:exec-role",
		Images:      map[string]string{},
		Environment: map[string]map[string]string{},
	}
	diffs := DiffTaskDefinitions(old, new)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d: %v", len(diffs), diffs)
	}
	if diffs[0].Field != "TaskRoleARN" || diffs[0].Kind != "changed" {
		t.Errorf("unexpected diff: %+v", diffs[0])
	}
}
