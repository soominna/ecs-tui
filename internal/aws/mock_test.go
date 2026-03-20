package aws

import (
	"context"

	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// MockECSAPI is a test double that implements ECSAPI.
// Set the function fields you need; unset fields return zero values.
type MockECSAPI struct {
	ListClustersFunc                    func(ctx context.Context) ([]ClusterInfo, error)
	ListServicesFunc                    func(ctx context.Context, cluster string) ([]ServiceInfo, error)
	ListTasksFunc                       func(ctx context.Context, cluster, service string, status ecstypes.DesiredStatus) ([]TaskInfo, error)
	ListTasksAllFunc                    func(ctx context.Context, cluster, service string) ([]TaskInfo, error)
	DescribeTaskDefinitionFunc          func(ctx context.Context, taskDefARN string) (*TaskDefinitionInfo, error)
	DescribeTaskDefinitionForContFunc   func(ctx context.Context, taskDefARN, containerName string) (*TaskDefinitionInfo, error)
	GetServiceEventsFunc                func(ctx context.Context, cluster, serviceName string) ([]ServiceEvent, error)
	GetServiceMetricsFunc               func(ctx context.Context, cluster string, serviceNames []string) (map[string]*ServiceMetrics, error)
	GetLogInfoFunc                      func(ctx context.Context, taskDefARN, containerName, taskID string) (*LogInfo, error)
	StartLiveTailFunc                   func(ctx context.Context, logGroupARN string, logStreamNames []string, filterPattern string, eventCh chan<- LogEvent) error
	GetLogEventsFunc                    func(ctx context.Context, logGroup, logStream, nextToken string, limit int32) ([]LogEvent, string, error)
	ForceNewDeploymentFunc              func(ctx context.Context, cluster, serviceName string) error
	UpdateDesiredCountFunc              func(ctx context.Context, cluster, serviceName string, count int32) error
	StopTaskFunc                        func(ctx context.Context, cluster, taskARN, reason string) error
	GetServiceDeploymentsFunc           func(ctx context.Context, cluster, serviceName string) (*ServiceDeploymentInfo, error)
	DescribeTaskDefinitionDetailFunc    func(ctx context.Context, taskDefARN string) (*TaskDefinitionDetail, error)
	GetServiceMetricsHistoryFunc        func(ctx context.Context, cluster string, serviceNames []string, dataPoints int32) (map[string]*ServiceMetricsHistory, error)
}

func (m *MockECSAPI) ListClusters(ctx context.Context) ([]ClusterInfo, error) {
	if m.ListClustersFunc != nil {
		return m.ListClustersFunc(ctx)
	}
	return nil, nil
}
func (m *MockECSAPI) ListServices(ctx context.Context, cluster string) ([]ServiceInfo, error) {
	if m.ListServicesFunc != nil {
		return m.ListServicesFunc(ctx, cluster)
	}
	return nil, nil
}
func (m *MockECSAPI) ListTasks(ctx context.Context, cluster, service string, status ecstypes.DesiredStatus) ([]TaskInfo, error) {
	if m.ListTasksFunc != nil {
		return m.ListTasksFunc(ctx, cluster, service, status)
	}
	return nil, nil
}
func (m *MockECSAPI) ListTasksAll(ctx context.Context, cluster, service string) ([]TaskInfo, error) {
	if m.ListTasksAllFunc != nil {
		return m.ListTasksAllFunc(ctx, cluster, service)
	}
	return nil, nil
}
func (m *MockECSAPI) DescribeTaskDefinition(ctx context.Context, taskDefARN string) (*TaskDefinitionInfo, error) {
	if m.DescribeTaskDefinitionFunc != nil {
		return m.DescribeTaskDefinitionFunc(ctx, taskDefARN)
	}
	return nil, nil
}
func (m *MockECSAPI) DescribeTaskDefinitionForContainer(ctx context.Context, taskDefARN, containerName string) (*TaskDefinitionInfo, error) {
	if m.DescribeTaskDefinitionForContFunc != nil {
		return m.DescribeTaskDefinitionForContFunc(ctx, taskDefARN, containerName)
	}
	return nil, nil
}
func (m *MockECSAPI) GetServiceEvents(ctx context.Context, cluster, serviceName string) ([]ServiceEvent, error) {
	if m.GetServiceEventsFunc != nil {
		return m.GetServiceEventsFunc(ctx, cluster, serviceName)
	}
	return nil, nil
}
func (m *MockECSAPI) GetServiceMetrics(ctx context.Context, cluster string, serviceNames []string) (map[string]*ServiceMetrics, error) {
	if m.GetServiceMetricsFunc != nil {
		return m.GetServiceMetricsFunc(ctx, cluster, serviceNames)
	}
	return nil, nil
}
func (m *MockECSAPI) GetLogInfo(ctx context.Context, taskDefARN, containerName, taskID string) (*LogInfo, error) {
	if m.GetLogInfoFunc != nil {
		return m.GetLogInfoFunc(ctx, taskDefARN, containerName, taskID)
	}
	return nil, nil
}
func (m *MockECSAPI) StartLiveTail(ctx context.Context, logGroupARN string, logStreamNames []string, filterPattern string, eventCh chan<- LogEvent) error {
	if m.StartLiveTailFunc != nil {
		return m.StartLiveTailFunc(ctx, logGroupARN, logStreamNames, filterPattern, eventCh)
	}
	return nil
}
func (m *MockECSAPI) GetLogEvents(ctx context.Context, logGroup, logStream, nextToken string, limit int32) ([]LogEvent, string, error) {
	if m.GetLogEventsFunc != nil {
		return m.GetLogEventsFunc(ctx, logGroup, logStream, nextToken, limit)
	}
	return nil, "", nil
}
func (m *MockECSAPI) ForceNewDeployment(ctx context.Context, cluster, serviceName string) error {
	if m.ForceNewDeploymentFunc != nil {
		return m.ForceNewDeploymentFunc(ctx, cluster, serviceName)
	}
	return nil
}
func (m *MockECSAPI) UpdateDesiredCount(ctx context.Context, cluster, serviceName string, count int32) error {
	if m.UpdateDesiredCountFunc != nil {
		return m.UpdateDesiredCountFunc(ctx, cluster, serviceName, count)
	}
	return nil
}
func (m *MockECSAPI) StopTask(ctx context.Context, cluster, taskARN, reason string) error {
	if m.StopTaskFunc != nil {
		return m.StopTaskFunc(ctx, cluster, taskARN, reason)
	}
	return nil
}
func (m *MockECSAPI) GetServiceDeployments(ctx context.Context, cluster, serviceName string) (*ServiceDeploymentInfo, error) {
	if m.GetServiceDeploymentsFunc != nil {
		return m.GetServiceDeploymentsFunc(ctx, cluster, serviceName)
	}
	return nil, nil
}
func (m *MockECSAPI) DescribeTaskDefinitionDetail(ctx context.Context, taskDefARN string) (*TaskDefinitionDetail, error) {
	if m.DescribeTaskDefinitionDetailFunc != nil {
		return m.DescribeTaskDefinitionDetailFunc(ctx, taskDefARN)
	}
	return nil, nil
}
func (m *MockECSAPI) GetServiceMetricsHistory(ctx context.Context, cluster string, serviceNames []string, dataPoints int32) (map[string]*ServiceMetricsHistory, error) {
	if m.GetServiceMetricsHistoryFunc != nil {
		return m.GetServiceMetricsHistoryFunc(ctx, cluster, serviceNames, dataPoints)
	}
	return nil, nil
}
