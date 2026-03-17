package aws

import (
	"context"
	"fmt"
	"time"

	awslib "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

type ServiceMetrics struct {
	CPUUtilization    *float64
	MemoryUtilization *float64
}

func (c *Client) GetServiceMetrics(ctx context.Context, cluster string, serviceNames []string) (map[string]*ServiceMetrics, error) {
	result := make(map[string]*ServiceMetrics)
	if len(serviceNames) == 0 {
		return result, nil
	}

	// Extract cluster name from ARN if needed
	clusterName := cluster

	endTime := time.Now()
	startTime := endTime.Add(-5 * time.Minute)

	queries := make([]cwtypes.MetricDataQuery, 0, len(serviceNames)*2)
	queryIDMap := make(map[string]string, len(serviceNames)*2) // queryID -> serviceName

	for i, svcName := range serviceNames {
		result[svcName] = &ServiceMetrics{}

		cpuID := fmt.Sprintf("cpu_%d", i)
		memID := fmt.Sprintf("mem_%d", i)
		queryIDMap[cpuID] = svcName
		queryIDMap[memID] = svcName

		queries = append(queries, cwtypes.MetricDataQuery{
			Id: awslib.String(cpuID),
			MetricStat: &cwtypes.MetricStat{
				Metric: &cwtypes.Metric{
					Namespace:  awslib.String("AWS/ECS"),
					MetricName: awslib.String("CPUUtilization"),
					Dimensions: []cwtypes.Dimension{
						{Name: awslib.String("ClusterName"), Value: awslib.String(clusterName)},
						{Name: awslib.String("ServiceName"), Value: awslib.String(svcName)},
					},
				},
				Period: awslib.Int32(300),
				Stat:   awslib.String("Average"),
			},
		})

		queries = append(queries, cwtypes.MetricDataQuery{
			Id: awslib.String(memID),
			MetricStat: &cwtypes.MetricStat{
				Metric: &cwtypes.Metric{
					Namespace:  awslib.String("AWS/ECS"),
					MetricName: awslib.String("MemoryUtilization"),
					Dimensions: []cwtypes.Dimension{
						{Name: awslib.String("ClusterName"), Value: awslib.String(clusterName)},
						{Name: awslib.String("ServiceName"), Value: awslib.String(svcName)},
					},
				},
				Period: awslib.Int32(300),
				Stat:   awslib.String("Average"),
			},
		})
	}

	// GetMetricData allows max 500 queries, batch if needed
	for i := 0; i < len(queries); i += 500 {
		end := i + 500
		if end > len(queries) {
			end = len(queries)
		}
		out, err := c.Metrics.GetMetricData(ctx, &cloudwatch.GetMetricDataInput{
			MetricDataQueries: queries[i:end],
			StartTime:         awslib.Time(startTime),
			EndTime:           awslib.Time(endTime),
		})
		if err != nil {
			return nil, fmt.Errorf("getting metrics: %w", err)
		}

		for _, r := range out.MetricDataResults {
			id := awslib.ToString(r.Id)
			svcName, ok := queryIDMap[id]
			if !ok || len(r.Values) == 0 {
				continue
			}
			val := r.Values[0]
			metrics := result[svcName]
			if len(id) >= 4 && id[:4] == "cpu_" {
				metrics.CPUUtilization = &val
			} else if len(id) >= 4 && id[:4] == "mem_" {
				metrics.MemoryUtilization = &val
			}
		}
	}

	return result, nil
}
