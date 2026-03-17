package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type ClusterInfo struct {
	ARN  string
	Name string
}

type ServiceInfo struct {
	Name         string
	Status       string
	RunningCount int32
	DesiredCount int32
	PendingCount int32
	TaskDef      string
	LastEvent    string
}

type TaskInfo struct {
	TaskID        string
	TaskARN       string
	Status        string
	IP            string
	StartedAt     *time.Time
	HealthStatus  string
	ContainerName string
	TaskDefARN    string
}

type TaskDefinitionInfo struct {
	Family    string
	CPU       string
	Memory    string
	LogGroup  string
	LogPrefix string
}

type ServiceEvent struct {
	CreatedAt time.Time
	Message   string
}

func (c *Client) ListClusters(ctx context.Context) ([]ClusterInfo, error) {
	var clusterARNs []string
	paginator := ecs.NewListClustersPaginator(c.ECS, &ecs.ListClustersInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing clusters: %w", err)
		}
		clusterARNs = append(clusterARNs, out.ClusterArns...)
	}

	if len(clusterARNs) == 0 {
		return nil, nil
	}

	desc, err := c.ECS.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: clusterARNs,
	})
	if err != nil {
		return nil, fmt.Errorf("describing clusters: %w", err)
	}

	clusters := make([]ClusterInfo, 0, len(desc.Clusters))
	for _, cl := range desc.Clusters {
		clusters = append(clusters, ClusterInfo{
			ARN:  aws.ToString(cl.ClusterArn),
			Name: aws.ToString(cl.ClusterName),
		})
	}
	return clusters, nil
}

func (c *Client) ListServices(ctx context.Context, cluster string) ([]ServiceInfo, error) {
	var serviceARNs []string
	paginator := ecs.NewListServicesPaginator(c.ECS, &ecs.ListServicesInput{
		Cluster: aws.String(cluster),
	})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing services: %w", err)
		}
		serviceARNs = append(serviceARNs, out.ServiceArns...)
	}

	if len(serviceARNs) == 0 {
		return nil, nil
	}

	// DescribeServices max 10 at a time
	var services []ServiceInfo
	for i := 0; i < len(serviceARNs); i += 10 {
		end := i + 10
		if end > len(serviceARNs) {
			end = len(serviceARNs)
		}
		desc, err := c.ECS.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  aws.String(cluster),
			Services: serviceARNs[i:end],
		})
		if err != nil {
			return nil, fmt.Errorf("describing services: %w", err)
		}
		for _, svc := range desc.Services {
			var lastEvent string
			if len(svc.Events) > 0 {
				lastEvent = aws.ToString(svc.Events[0].Message)
				if len(lastEvent) > 100 {
					lastEvent = lastEvent[:100] + "..."
				}
			}
			taskDef := shortTaskDef(aws.ToString(svc.TaskDefinition))
			services = append(services, ServiceInfo{
				Name:         aws.ToString(svc.ServiceName),
				Status:       aws.ToString(svc.Status),
				RunningCount: svc.RunningCount,
				DesiredCount: svc.DesiredCount,
				PendingCount: svc.PendingCount,
				TaskDef:      taskDef,
				LastEvent:    lastEvent,
			})
		}
	}
	return services, nil
}

func (c *Client) ListTasks(ctx context.Context, cluster, service string) ([]TaskInfo, error) {
	var taskARNs []string
	paginator := ecs.NewListTasksPaginator(c.ECS, &ecs.ListTasksInput{
		Cluster:     aws.String(cluster),
		ServiceName: aws.String(service),
	})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing tasks: %w", err)
		}
		taskARNs = append(taskARNs, out.TaskArns...)
	}

	if len(taskARNs) == 0 {
		return nil, nil
	}

	// DescribeTasks max 100 at a time
	var tasks []TaskInfo
	for i := 0; i < len(taskARNs); i += 100 {
		end := i + 100
		if end > len(taskARNs) {
			end = len(taskARNs)
		}
		desc, err := c.ECS.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: aws.String(cluster),
			Tasks:   taskARNs[i:end],
		})
		if err != nil {
			return nil, fmt.Errorf("describing tasks: %w", err)
		}
		for _, t := range desc.Tasks {
			taskARN := aws.ToString(t.TaskArn)
			taskID := extractTaskID(taskARN)
			var ip, containerName, healthStatus string
			if len(t.Containers) > 0 {
				containerName = aws.ToString(t.Containers[0].Name)
				healthStatus = string(t.Containers[0].HealthStatus)
				for _, ni := range t.Containers[0].NetworkInterfaces {
					if ni.PrivateIpv4Address != nil {
						ip = aws.ToString(ni.PrivateIpv4Address)
						break
					}
				}
			}
			if ip == "" && t.Attachments != nil {
				for _, att := range t.Attachments {
					for _, detail := range att.Details {
						if aws.ToString(detail.Name) == "privateIPv4Address" {
							ip = aws.ToString(detail.Value)
						}
					}
				}
			}
			if healthStatus == "" || healthStatus == "UNKNOWN" {
				healthStatus = string(t.HealthStatus)
			}
			tasks = append(tasks, TaskInfo{
				TaskID:        taskID,
				TaskARN:       taskARN,
				Status:        aws.ToString(t.LastStatus),
				IP:            ip,
				StartedAt:     t.StartedAt,
				HealthStatus:  healthStatus,
				ContainerName: containerName,
				TaskDefARN:    aws.ToString(t.TaskDefinitionArn),
			})
		}
	}
	return tasks, nil
}

// describeTaskDef is a shared helper that calls DescribeTaskDefinition and
// returns the raw output along with a partially filled TaskDefinitionInfo.
func (c *Client) describeTaskDef(ctx context.Context, taskDefARN string) (*ecs.DescribeTaskDefinitionOutput, *TaskDefinitionInfo, error) {
	out, err := c.ECS.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefARN),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("describing task definition: %w", err)
	}
	td := out.TaskDefinition
	info := &TaskDefinitionInfo{
		Family: aws.ToString(td.Family),
		CPU:    aws.ToString(td.Cpu),
		Memory: aws.ToString(td.Memory),
	}
	return out, info, nil
}

// applyLogConfig sets LogGroup/LogPrefix from the first awslogs container definition.
func applyLogConfig(info *TaskDefinitionInfo, containers []ecstypes.ContainerDefinition) {
	for _, cd := range containers {
		if cd.LogConfiguration != nil && cd.LogConfiguration.LogDriver == ecstypes.LogDriverAwslogs {
			opts := cd.LogConfiguration.Options
			info.LogGroup = opts["awslogs-group"]
			info.LogPrefix = opts["awslogs-stream-prefix"]
			return
		}
	}
}

func (c *Client) DescribeTaskDefinition(ctx context.Context, taskDefARN string) (*TaskDefinitionInfo, error) {
	out, info, err := c.describeTaskDef(ctx, taskDefARN)
	if err != nil {
		return nil, err
	}
	applyLogConfig(info, out.TaskDefinition.ContainerDefinitions)
	return info, nil
}

// DescribeTaskDefinitionForContainer returns task def info with log config
// matched to a specific container name.
func (c *Client) DescribeTaskDefinitionForContainer(ctx context.Context, taskDefARN, containerName string) (*TaskDefinitionInfo, error) {
	out, info, err := c.describeTaskDef(ctx, taskDefARN)
	if err != nil {
		return nil, err
	}

	// First try: match by container name
	for _, cd := range out.TaskDefinition.ContainerDefinitions {
		if aws.ToString(cd.Name) == containerName {
			if cd.LogConfiguration != nil && cd.LogConfiguration.LogDriver == ecstypes.LogDriverAwslogs {
				opts := cd.LogConfiguration.Options
				info.LogGroup = opts["awslogs-group"]
				info.LogPrefix = opts["awslogs-stream-prefix"]
			}
			return info, nil
		}
	}

	// Fallback: use first container with awslogs
	applyLogConfig(info, out.TaskDefinition.ContainerDefinitions)
	return info, nil
}

func (c *Client) GetServiceEvents(ctx context.Context, cluster, serviceName string) ([]ServiceEvent, error) {
	out, err := c.ECS.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(cluster),
		Services: []string{serviceName},
	})
	if err != nil {
		return nil, fmt.Errorf("describing service for events: %w", err)
	}
	if len(out.Services) == 0 {
		return nil, fmt.Errorf("service %s not found", serviceName)
	}

	events := make([]ServiceEvent, 0, len(out.Services[0].Events))
	for _, e := range out.Services[0].Events {
		events = append(events, ServiceEvent{
			CreatedAt: aws.ToTime(e.CreatedAt),
			Message:   aws.ToString(e.Message),
		})
	}
	return events, nil
}

func (c *Client) ForceNewDeployment(ctx context.Context, cluster, serviceName string) error {
	_, err := c.ECS.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:            aws.String(cluster),
		Service:            aws.String(serviceName),
		ForceNewDeployment: true,
	})
	if err != nil {
		return fmt.Errorf("force new deployment: %w", err)
	}
	return nil
}

func (c *Client) UpdateDesiredCount(ctx context.Context, cluster, serviceName string, count int32) error {
	_, err := c.ECS.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:      aws.String(cluster),
		Service:      aws.String(serviceName),
		DesiredCount: aws.Int32(count),
	})
	if err != nil {
		return fmt.Errorf("update desired count: %w", err)
	}
	return nil
}

func (c *Client) StopTask(ctx context.Context, cluster, taskARN, reason string) error {
	_, err := c.ECS.StopTask(ctx, &ecs.StopTaskInput{
		Cluster: aws.String(cluster),
		Task:    aws.String(taskARN),
		Reason:  aws.String(reason),
	})
	if err != nil {
		return fmt.Errorf("stop task: %w", err)
	}
	return nil
}

func extractTaskID(taskARN string) string {
	parts := strings.Split(taskARN, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return taskARN
}

func shortTaskDef(arn string) string {
	parts := strings.Split(arn, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return arn
}
