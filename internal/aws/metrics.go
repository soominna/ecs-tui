package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	awslib "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// extractClusterName extracts the cluster name from an ARN (e.g. "arn:aws:ecs:...:cluster/name")
// or returns the input as-is if it's already a plain name.
func extractClusterName(cluster string) string {
	if i := strings.LastIndex(cluster, "/"); i >= 0 {
		return cluster[i+1:]
	}
	return cluster
}

type ServiceMetrics struct {
	CPUUtilization    *float64
	MemoryUtilization *float64
}

func (c *Client) GetServiceMetrics(ctx context.Context, cluster string, serviceNames []string) (map[string]*ServiceMetrics, error) {
	result := make(map[string]*ServiceMetrics)
	if len(serviceNames) == 0 {
		return result, nil
	}

	clusterName := extractClusterName(cluster)

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

// ServiceMetricsHistory holds time-series metric values for sparkline display.
type ServiceMetricsHistory struct {
	CPUValues    []float64 // dataPoints values (5-min intervals)
	MemoryValues []float64
}

// GetServiceMetricsHistory fetches historical metric data points for sparkline rendering.
func (c *Client) GetServiceMetricsHistory(ctx context.Context, cluster string, serviceNames []string, dataPoints int32) (map[string]*ServiceMetricsHistory, error) {
	result := make(map[string]*ServiceMetricsHistory)
	if len(serviceNames) == 0 || dataPoints <= 0 {
		return result, nil
	}

	clusterName := extractClusterName(cluster)
	endTime := time.Now()
	startTime := endTime.Add(-time.Duration(dataPoints) * 5 * time.Minute)

	queries := make([]cwtypes.MetricDataQuery, 0, len(serviceNames)*2)
	queryIDMap := make(map[string]string, len(serviceNames)*2)

	for i, svcName := range serviceNames {
		result[svcName] = &ServiceMetricsHistory{}

		cpuID := fmt.Sprintf("hcpu_%d", i)
		memID := fmt.Sprintf("hmem_%d", i)
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
			return nil, fmt.Errorf("getting metrics history: %w", err)
		}

		for _, r := range out.MetricDataResults {
			id := awslib.ToString(r.Id)
			svcName, ok := queryIDMap[id]
			if !ok || len(r.Values) == 0 {
				continue
			}
			// CloudWatch returns newest-first; reverse for chronological order
			vals := make([]float64, len(r.Values))
			for j := range r.Values {
				vals[len(r.Values)-1-j] = r.Values[j]
			}
			hist := result[svcName]
			if len(id) >= 5 && id[:5] == "hcpu_" {
				hist.CPUValues = vals
			} else if len(id) >= 5 && id[:5] == "hmem_" {
				hist.MemoryValues = vals
			}
		}
	}

	return result, nil
}
