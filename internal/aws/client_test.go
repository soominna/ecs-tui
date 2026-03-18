package aws

import "testing"

func TestCommonRegions(t *testing.T) {
	regions := CommonRegions()
	if len(regions) == 0 {
		t.Fatal("CommonRegions() returned empty slice")
	}

	// Verify well-known regions are included
	expected := map[string]bool{
		"us-east-1":      false,
		"us-west-2":      false,
		"ap-northeast-1": false,
		"ap-northeast-2": false,
		"eu-west-1":      false,
	}

	for _, r := range regions {
		if _, ok := expected[r]; ok {
			expected[r] = true
		}
	}

	for region, found := range expected {
		if !found {
			t.Errorf("CommonRegions() missing expected region %q", region)
		}
	}
}

func TestRegionPattern(t *testing.T) {
	valid := []string{
		"us-east-1",
		"ap-northeast-2",
		"eu-central-1",
		"cn-northwest-1",
		"us-gov-west-1",
	}
	invalid := []string{
		"",
		"us-east",
		"invalid",
		"US-EAST-1",
		"123-456-7",
	}

	for _, r := range valid {
		if !regionPattern.MatchString(r) {
			t.Errorf("regionPattern should match %q", r)
		}
	}
	for _, r := range invalid {
		if regionPattern.MatchString(r) {
			t.Errorf("regionPattern should not match %q", r)
		}
	}
}
