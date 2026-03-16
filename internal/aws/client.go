package aws

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

type Client struct {
	ECS     *ecs.Client
	Logs    *cloudwatchlogs.Client
	Metrics *cloudwatch.Client
	Profile string
	Region  string
}

func NewClient(ctx context.Context, profile, region string) (*Client, error) {
	var opts []func(*config.LoadOptions) error

	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	if region == "" {
		region = cfg.Region
	}

	return &Client{
		ECS:     ecs.NewFromConfig(cfg),
		Logs:    cloudwatchlogs.NewFromConfig(cfg),
		Metrics: cloudwatch.NewFromConfig(cfg),
		Profile: profile,
		Region:  region,
	}, nil
}

func ListProfiles() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	var profiles []string
	for _, fname := range []string{"config", "credentials"} {
		p := filepath.Join(home, ".aws", fname)
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
				section := strings.Trim(line, "[]")
				section = strings.TrimPrefix(section, "profile ")
				if section != "" {
					profiles = append(profiles, section)
				}
			}
		}
		f.Close()
	}

	seen := make(map[string]bool)
	var unique []string
	for _, p := range profiles {
		if !seen[p] {
			seen[p] = true
			unique = append(unique, p)
		}
	}
	return unique, nil
}

// DetectCurrentConfig returns the currently active profile and region
// from environment variables and AWS CLI defaults.
func DetectCurrentConfig() (profile, region string) {
	// Check environment variables first
	profile = os.Getenv("AWS_PROFILE")
	if profile == "" {
		profile = os.Getenv("AWS_DEFAULT_PROFILE")
	}
	if profile == "" {
		profile = "default"
	}

	region = os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}

	// If no region from env, try to read from config file
	if region == "" {
		region = readRegionFromConfig(profile)
	}

	return profile, region
}

func readRegionFromConfig(profile string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	f, err := os.Open(filepath.Join(home, ".aws", "config"))
	if err != nil {
		return ""
	}
	defer f.Close()

	// Find the section for the profile
	targetSection := "[default]"
	if profile != "default" {
		targetSection = "[profile " + profile + "]"
	}

	scanner := bufio.NewScanner(f)
	inSection := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") {
			inSection = (line == targetSection)
			continue
		}
		if inSection && strings.HasPrefix(line, "region") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

func CommonRegions() []string {
	return []string{
		"us-east-1",
		"us-east-2",
		"us-west-1",
		"us-west-2",
		"ap-northeast-1",
		"ap-northeast-2",
		"ap-northeast-3",
		"ap-southeast-1",
		"ap-southeast-2",
		"ap-south-1",
		"eu-west-1",
		"eu-west-2",
		"eu-west-3",
		"eu-central-1",
		"eu-north-1",
		"sa-east-1",
		"ca-central-1",
	}
}

func StringVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func StringPtr(s string) *string {
	return aws.String(s)
}
