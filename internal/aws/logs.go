package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	awslib "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	logstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

type LogEvent struct {
	Timestamp time.Time
	Message   string
}

type LogInfo struct {
	LogGroup    string
	LogGroupARN string
	LogStream   string
}

func (c *Client) GetLogInfo(ctx context.Context, taskDefARN, containerName, taskID string) (*LogInfo, error) {
	td, err := c.DescribeTaskDefinitionForContainer(ctx, taskDefARN, containerName)
	if err != nil {
		return nil, err
	}

	if td.LogGroup == "" {
		return nil, fmt.Errorf("no awslogs configuration found for container %q", containerName)
	}

	logStream := ""
	if td.LogPrefix != "" && containerName != "" && taskID != "" {
		logStream = fmt.Sprintf("%s/%s/%s", td.LogPrefix, containerName, taskID)
	}

	// Get the log group ARN
	descOut, err := c.Logs.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: awslib.String(td.LogGroup),
		Limit:              awslib.Int32(1),
	})
	if err != nil {
		return nil, fmt.Errorf("describing log group: %w", err)
	}

	var logGroupARN string
	for _, lg := range descOut.LogGroups {
		if awslib.ToString(lg.LogGroupName) == td.LogGroup {
			logGroupARN = awslib.ToString(lg.Arn)
			break
		}
	}

	// Ensure ARN ends with :* for LiveTail
	if logGroupARN != "" {
		if !strings.HasSuffix(logGroupARN, ":*") {
			logGroupARN = strings.TrimSuffix(logGroupARN, ":") + ":*"
		}
	}

	// If no logStream but we have logGroup, try to discover the stream
	if logStream == "" && td.LogGroup != "" {
		streams, err := c.Logs.DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName: awslib.String(td.LogGroup),
			OrderBy:      logstypes.OrderByLastEventTime,
			Descending:   awslib.Bool(true),
			Limit:        awslib.Int32(10),
		})
		if err == nil && len(streams.LogStreams) > 0 {
			// Try to find a stream matching this taskID
			for _, s := range streams.LogStreams {
				name := awslib.ToString(s.LogStreamName)
				if strings.Contains(name, taskID) {
					logStream = name
					break
				}
			}
			// If no match found, use the most recent stream
			if logStream == "" {
				logStream = awslib.ToString(streams.LogStreams[0].LogStreamName)
			}
		}
	}

	return &LogInfo{
		LogGroup:    td.LogGroup,
		LogGroupARN: logGroupARN,
		LogStream:   logStream,
	}, nil
}

func (c *Client) StartLiveTail(ctx context.Context, logGroupARN string, logStreamNames []string, filterPattern string, eventCh chan<- LogEvent) error {
	input := &cloudwatchlogs.StartLiveTailInput{
		LogGroupIdentifiers: []string{logGroupARN},
	}
	if len(logStreamNames) > 0 {
		input.LogStreamNames = logStreamNames
	}
	if filterPattern != "" {
		input.LogEventFilterPattern = awslib.String(filterPattern)
	}

	resp, err := c.Logs.StartLiveTail(ctx, input)
	if err != nil {
		return fmt.Errorf("starting live tail: %w", err)
	}

	stream := resp.GetStream()
	eventsCh := stream.Events()
	go func() {
		defer stream.Close()
		defer close(eventCh)
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-eventsCh:
				if !ok {
					return
				}
				switch v := event.(type) {
				case *logstypes.StartLiveTailResponseStreamMemberSessionUpdate:
					for _, le := range v.Value.SessionResults {
						eventCh <- LogEvent{
							Timestamp: time.UnixMilli(awslib.ToInt64(le.Timestamp)),
							Message:   awslib.ToString(le.Message),
						}
					}
				}
			}
		}
	}()

	return nil
}

func (c *Client) GetLogEvents(ctx context.Context, logGroup, logStream, nextToken string, limit int32) ([]LogEvent, string, error) {
	input := &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  awslib.String(logGroup),
		LogStreamName: awslib.String(logStream),
		StartFromHead: awslib.Bool(false),
		Limit:         awslib.Int32(limit),
	}
	if nextToken != "" {
		input.NextToken = awslib.String(nextToken)
	}

	out, err := c.Logs.GetLogEvents(ctx, input)
	if err != nil {
		return nil, "", fmt.Errorf("getting log events: %w", err)
	}

	events := make([]LogEvent, 0, len(out.Events))
	for _, e := range out.Events {
		events = append(events, LogEvent{
			Timestamp: time.UnixMilli(awslib.ToInt64(e.Timestamp)),
			Message:   awslib.ToString(e.Message),
		})
	}

	var newToken string
	if out.NextForwardToken != nil {
		newToken = awslib.ToString(out.NextForwardToken)
	}

	return events, newToken, nil
}
