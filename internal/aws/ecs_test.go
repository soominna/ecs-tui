package aws

import "testing"

func TestExtractTaskID(t *testing.T) {
	tests := []struct {
		name string
		arn  string
		want string
	}{
		{
			name: "full ARN",
			arn:  "arn:aws:ecs:us-east-1:123456789012:task/my-cluster/abc123def456",
			want: "abc123def456",
		},
		{
			name: "short ARN",
			arn:  "task/my-cluster/abc123",
			want: "abc123",
		},
		{
			name: "just ID",
			arn:  "abc123",
			want: "abc123",
		},
		{
			name: "empty string",
			arn:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTaskID(tt.arn)
			if got != tt.want {
				t.Errorf("extractTaskID(%q) = %q, want %q", tt.arn, got, tt.want)
			}
		})
	}
}

func TestShortTaskDef(t *testing.T) {
	tests := []struct {
		name string
		arn  string
		want string
	}{
		{
			name: "full ARN",
			arn:  "arn:aws:ecs:us-east-1:123456789012:task-definition/my-app:5",
			want: "my-app:5",
		},
		{
			name: "short form",
			arn:  "task-definition/my-app:3",
			want: "my-app:3",
		},
		{
			name: "already short",
			arn:  "my-app:1",
			want: "my-app:1",
		},
		{
			name: "empty",
			arn:  "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortTaskDef(tt.arn)
			if got != tt.want {
				t.Errorf("shortTaskDef(%q) = %q, want %q", tt.arn, got, tt.want)
			}
		})
	}
}
