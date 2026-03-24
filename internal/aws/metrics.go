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

// newMetricQuery builds a single MetricDataQuery for an ECS service metric.
func newMetricQuery(id, metricName, clusterName, svcName string, period int32) cwtypes.MetricDataQuery {
	return cwtypes.MetricDataQuery{
		Id: awslib.String(id),
		MetricStat: &cwtypes.MetricStat{
			Metric: &cwtypes.Metric{
				Namespace:  awslib.String("AWS/ECS"),
				MetricName: awslib.String(metricName),
				Dimensions: []cwtypes.Dimension{
					{Name: awslib.String("ClusterName"), Value: awslib.String(clusterName)},
					{Name: awslib.String("ServiceName"), Value: awslib.String(svcName)},
				},
			},
			Period: awslib.Int32(period),
			Stat:   awslib.String("Average"),
		},
	}
}

// buildMetricQueries creates CPU and Memory metric queries for each service.
// prefix differentiates current ("") vs history ("h") queries.
func buildMetricQueries(clusterName string, serviceNames []string, prefix string, period int32) ([]cwtypes.MetricDataQuery, map[string]string) {
	queries := make([]cwtypes.MetricDataQuery, 0, len(serviceNames)*2)
	queryIDMap := make(map[string]string, len(serviceNames)*2)

	for i, svcName := range serviceNames {
		cpuID := fmt.Sprintf("%scpu_%d", prefix, i)
		memID := fmt.Sprintf("%smem_%d", prefix, i)
		queryIDMap[cpuID] = svcName
		queryIDMap[memID] = svcName

		queries = append(queries,
			newMetricQuery(cpuID, "CPUUtilization", clusterName, svcName, period),
			newMetricQuery(memID, "MemoryUtilization", clusterName, svcName, period),
		)
	}
	return queries, queryIDMap
}

// executeMetricQueries runs GetMetricData in batches of 500.
func executeMetricQueries(ctx context.Context, client *cloudwatch.Client, queries []cwtypes.MetricDataQuery, startTime, endTime time.Time) ([]cwtypes.MetricDataResult, error) {
	var results []cwtypes.MetricDataResult
	for i := 0; i < len(queries); i += 500 {
		end := i + 500
		if end > len(queries) {
			end = len(queries)
		}
		out, err := client.GetMetricData(ctx, &cloudwatch.GetMetricDataInput{
			MetricDataQueries: queries[i:end],
			StartTime:         awslib.Time(startTime),
			EndTime:           awslib.Time(endTime),
		})
		if err != nil {
			return nil, err
		}
		results = append(results, out.MetricDataResults...)
	}
	return results, nil
}

func (c *Client) GetServiceMetrics(ctx context.Context, cluster string, serviceNames []string) (map[string]*ServiceMetrics, error) {
	result := make(map[string]*ServiceMetrics)
	if len(serviceNames) == 0 {
		return result, nil
	}

	clusterName := extractClusterName(cluster)
	endTime := time.Now()
	startTime := endTime.Add(-5 * time.Minute)

	queries, queryIDMap := buildMetricQueries(clusterName, serviceNames, "", 300)
	for _, svcName := range serviceNames {
		result[svcName] = &ServiceMetrics{}
	}

	metricResults, err := executeMetricQueries(ctx, c.Metrics, queries, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("getting metrics: %w", err)
	}

	for _, r := range metricResults {
		id := awslib.ToString(r.Id)
		svcName, ok := queryIDMap[id]
		if !ok || len(r.Values) == 0 {
			continue
		}
		val := r.Values[0]
		metrics := result[svcName]
		if strings.HasPrefix(id, "cpu_") {
			metrics.CPUUtilization = &val
		} else if strings.HasPrefix(id, "mem_") {
			metrics.MemoryUtilization = &val
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

	queries, queryIDMap := buildMetricQueries(clusterName, serviceNames, "h", 300)
	for _, svcName := range serviceNames {
		result[svcName] = &ServiceMetricsHistory{}
	}

	metricResults, err := executeMetricQueries(ctx, c.Metrics, queries, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("getting metrics history: %w", err)
	}

	for _, r := range metricResults {
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
		if strings.HasPrefix(id, "hcpu_") {
			hist.CPUValues = vals
		} else if strings.HasPrefix(id, "hmem_") {
			hist.MemoryValues = vals
		}
	}

	return result, nil
}
