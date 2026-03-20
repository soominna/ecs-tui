package aws

import (
	"fmt"
	"sort"
)

// DiffEntry represents a single difference between two task definitions.
type DiffEntry struct {
	Field    string // e.g. "Image (web)", "Env.API_KEY (web)", "Memory"
	OldValue string
	NewValue string
	Kind     string // "changed", "added", "removed"
}

// DiffTaskDefinitions compares two TaskDefinitionDetail structs and returns
// a list of differences. This is a pure function with no AWS dependency.
func DiffTaskDefinitions(old, new *TaskDefinitionDetail) []DiffEntry {
	if old == nil || new == nil {
		return nil
	}

	var diffs []DiffEntry

	// Top-level fields
	if old.CPU != new.CPU {
		diffs = append(diffs, DiffEntry{Field: "CPU", OldValue: old.CPU, NewValue: new.CPU, Kind: "changed"})
	}
	if old.Memory != new.Memory {
		diffs = append(diffs, DiffEntry{Field: "Memory", OldValue: old.Memory, NewValue: new.Memory, Kind: "changed"})
	}
	if old.TaskRoleARN != new.TaskRoleARN {
		diffs = append(diffs, DiffEntry{Field: "TaskRoleARN", OldValue: old.TaskRoleARN, NewValue: new.TaskRoleARN, Kind: "changed"})
	}
	if old.ExecRoleARN != new.ExecRoleARN {
		diffs = append(diffs, DiffEntry{Field: "ExecRoleARN", OldValue: old.ExecRoleARN, NewValue: new.ExecRoleARN, Kind: "changed"})
	}

	// Collect all container names from both
	containers := make(map[string]bool)
	for name := range old.Images {
		containers[name] = true
	}
	for name := range new.Images {
		containers[name] = true
	}
	for name := range old.Environment {
		containers[name] = true
	}
	for name := range new.Environment {
		containers[name] = true
	}

	sorted := sortedKeys(containers)
	for _, cname := range sorted {
		// Image diff
		oldImg := old.Images[cname]
		newImg := new.Images[cname]
		if oldImg != newImg {
			field := fmt.Sprintf("Image (%s)", cname)
			switch {
			case oldImg == "":
				diffs = append(diffs, DiffEntry{Field: field, NewValue: newImg, Kind: "added"})
			case newImg == "":
				diffs = append(diffs, DiffEntry{Field: field, OldValue: oldImg, Kind: "removed"})
			default:
				diffs = append(diffs, DiffEntry{Field: field, OldValue: oldImg, NewValue: newImg, Kind: "changed"})
			}
		}

		// Environment diff
		oldEnv := old.Environment[cname]
		newEnv := new.Environment[cname]
		envKeys := make(map[string]bool)
		for k := range oldEnv {
			envKeys[k] = true
		}
		for k := range newEnv {
			envKeys[k] = true
		}
		for _, key := range sortedKeys(envKeys) {
			oldVal := oldEnv[key]
			newVal := newEnv[key]
			if oldVal != newVal {
				field := fmt.Sprintf("Env.%s (%s)", key, cname)
				switch {
				case oldVal == "":
					diffs = append(diffs, DiffEntry{Field: field, NewValue: newVal, Kind: "added"})
				case newVal == "":
					diffs = append(diffs, DiffEntry{Field: field, OldValue: oldVal, Kind: "removed"})
				default:
					diffs = append(diffs, DiffEntry{Field: field, OldValue: oldVal, NewValue: newVal, Kind: "changed"})
				}
			}
		}
	}

	return diffs
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
