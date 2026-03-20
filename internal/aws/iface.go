package aws

import (
	"context"

	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// ECSAPI is the minimal interface of Client methods used by the UI layer.
// Its sole purpose is to allow mock implementations in tests.
type ECSAPI interface {
	ListClusters(ctx context.Context) ([]ClusterInfo, error)
	ListServices(ctx context.Context, cluster string) ([]ServiceInfo, error)
	ListTasks(ctx context.Context, cluster, service string, status ecstypes.DesiredStatus) ([]TaskInfo, error)
	ListTasksAll(ctx context.Context, cluster, service string) ([]TaskInfo, error)
	DescribeTaskDefinition(ctx context.Context, taskDefARN string) (*TaskDefinitionInfo, error)
	DescribeTaskDefinitionForContainer(ctx context.Context, taskDefARN, containerName string) (*TaskDefinitionInfo, error)
	GetServiceEvents(ctx context.Context, cluster, serviceName string) ([]ServiceEvent, error)
	GetServiceMetrics(ctx context.Context, cluster string, serviceNames []string) (map[string]*ServiceMetrics, error)
	GetLogInfo(ctx context.Context, taskDefARN, containerName, taskID string) (*LogInfo, error)
	StartLiveTail(ctx context.Context, logGroupARN string, logStreamNames []string, filterPattern string, eventCh chan<- LogEvent) error
	GetLogEvents(ctx context.Context, logGroup, logStream, nextToken string, limit int32) ([]LogEvent, string, error)
	ForceNewDeployment(ctx context.Context, cluster, serviceName string) error
	UpdateDesiredCount(ctx context.Context, cluster, serviceName string, count int32) error
	StopTask(ctx context.Context, cluster, taskARN, reason string) error
	GetServiceDeployments(ctx context.Context, cluster, serviceName string) (*ServiceDeploymentInfo, error)
	DescribeTaskDefinitionDetail(ctx context.Context, taskDefARN string) (*TaskDefinitionDetail, error)
	GetServiceMetricsHistory(ctx context.Context, cluster string, serviceNames []string, dataPoints int32) (map[string]*ServiceMetricsHistory, error)
}
