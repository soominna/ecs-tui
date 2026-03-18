# ECS-TUI 개선 구현 계획서

> 작성일: 2026-03-18
> 대상 프로젝트: `ecs-tui` (Go + Bubbletea TUI)
> 예상 난이도: LOW (각 기능 독립적, 기존 패턴 활용)

---

## 목차

1. [Feature 1: Stopped Tasks 조회](#feature-1-stopped-tasks-조회)
2. [Feature 2: Config 파일 지원](#feature-2-config-파일-지원)
3. [Feature 3: 자동 새로고침 간격 설정](#feature-3-자동-새로고침-간격-설정)
4. [구현 순서 권장사항](#구현-순서-권장사항)

---

## Feature 1: Stopped Tasks 조회

### 개요

현재 `ListTasks`는 AWS API의 기본값인 RUNNING 태스크만 조회한다. STOPPED 태스크도 볼 수 있도록 토글 기능을 추가한다.

### 수정 파일 목록

| 파일 | 변경 유형 | 설명 |
|------|----------|------|
| `internal/aws/ecs.go` | 수정 | `ListTasks` 시그니처에 `DesiredStatus` 파라미터 추가 |
| `internal/ui/tasks.go` | 수정 | 필터 상태 필드 추가, `t` 키 핸들러, 헤더 표시 |

### 상세 변경사항

#### 1-1. `internal/aws/ecs.go` - ListTasks 시그니처 변경

**현재 코드 (145~150행):**
```go
func (c *Client) ListTasks(ctx context.Context, cluster, service string) ([]TaskInfo, error) {
    var taskARNs []string
    paginator := ecs.NewListTasksPaginator(c.ECS, &ecs.ListTasksInput{
        Cluster:     aws.String(cluster),
        ServiceName: aws.String(service),
    })
```

**변경 후:**
```go
func (c *Client) ListTasks(ctx context.Context, cluster, service string, desiredStatus ecstypes.DesiredStatus) ([]TaskInfo, error) {
    input := &ecs.ListTasksInput{
        Cluster:     aws.String(cluster),
        ServiceName: aws.String(service),
    }
    // DesiredStatus가 빈 문자열이 아닌 경우에만 설정
    // 빈 문자열이면 API 기본값(RUNNING)을 사용
    if desiredStatus != "" {
        input.DesiredStatus = desiredStatus
    }
    var taskARNs []string
    paginator := ecs.NewListTasksPaginator(c.ECS, input)
```

**핵심 사항:**
- AWS ECS `ListTasks` API의 `DesiredStatus` 필드는 `ecstypes.DesiredStatus` 타입이며, `ecstypes.DesiredStatusRunning` 또는 `ecstypes.DesiredStatusStopped` 값을 가진다.
- AWS API는 "ALL"을 직접 지원하지 않는다. ALL 모드에서는 RUNNING과 STOPPED를 각각 호출하여 결과를 합쳐야 한다.
- `ecstypes` import는 이미 12행에 존재: `ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"`

**ALL 모드 구현 (ListTasks 내부 또는 별도 헬퍼):**
```go
// ALL 상태 조회를 위한 헬퍼 함수
func (c *Client) ListTasksAll(ctx context.Context, cluster, service string) ([]TaskInfo, error) {
    running, err := c.ListTasks(ctx, cluster, service, ecstypes.DesiredStatusRunning)
    if err != nil {
        return nil, err
    }
    stopped, err := c.ListTasks(ctx, cluster, service, ecstypes.DesiredStatusStopped)
    if err != nil {
        return nil, err
    }
    return append(running, stopped...), nil
}
```

#### 1-2. `internal/ui/tasks.go` - TaskView에 필터 상태 토글 추가

**TaskView 구조체 필드 추가 (18~35행 부근):**
```go
type TaskView struct {
    // ... 기존 필드 ...
    // Task status filter
    taskStatusFilter ecstypes.DesiredStatus // "", "RUNNING", "STOPPED"
}
```

**초기값:** `taskStatusFilter`의 기본값은 `ecstypes.DesiredStatusRunning` (빈 문자열은 API 기본값이 RUNNING이므로 명시적으로 설정).

**NewTaskView 함수 수정 (47~58행):**
```go
func NewTaskView(client *awsclient.Client, cluster, service string) *TaskView {
    ti := textinput.New()
    ti.Placeholder = "Filter tasks..."
    ti.CharLimit = 50

    return &TaskView{
        client:           client,
        cluster:          cluster,
        service:          service,
        filterInput:      ti,
        taskStatusFilter: ecstypes.DesiredStatusRunning, // 기본값: RUNNING만 표시
    }
}
```

**fetchTasks 수정 (96~109행):**
```go
func (v *TaskView) fetchTasks() tea.Cmd {
    client := v.client
    cluster := v.cluster
    service := v.service
    statusFilter := v.taskStatusFilter
    return func() tea.Msg {
        ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
        defer cancel()
        var tasks []awsclient.TaskInfo
        var err error
        if statusFilter == "" {
            // ALL 모드: RUNNING + STOPPED 합치기
            tasks, err = client.ListTasksAll(ctx, cluster, service)
        } else {
            tasks, err = client.ListTasks(ctx, cluster, service, statusFilter)
        }
        if err != nil {
            return ErrorMsg{Err: err}
        }
        return tasksLoadedMsg{tasks: tasks}
    }
}
```

**키 핸들러 추가 - Update 함수 내 Normal mode (217행 부근, `switch msg.String()` 블록에 추가):**
```go
case "t":
    // 상태 필터 순환: RUNNING -> STOPPED -> ALL -> RUNNING
    switch v.taskStatusFilter {
    case ecstypes.DesiredStatusRunning:
        v.taskStatusFilter = ecstypes.DesiredStatusStopped
    case ecstypes.DesiredStatusStopped:
        v.taskStatusFilter = "" // ALL
    default:
        v.taskStatusFilter = ecstypes.DesiredStatusRunning
    }
    v.loaded = false // 로딩 표시
    return v, v.fetchTasks()
```

**ShortcutHelp에 `t` 키 추가 (75~84행):**
```go
return []Shortcut{
    {Key: "Enter/d", Desc: "Detail"},
    {Key: "l", Desc: "Logs"},
    {Key: "e", Desc: "Exec"},
    {Key: "s", Desc: "Stop Task"},
    {Key: "t", Desc: "Status Filter"},  // 추가
    {Key: "/", Desc: "Filter"},
    {Key: "r", Desc: "Refresh"},
    {Key: "Esc", Desc: "Back"},
}
```

**View 함수에 현재 필터 상태 표시 (292행 부근, `Updated ... ago` 라인 다음에 추가):**
```go
// 상태 필터 표시
statusLabel := "RUNNING"
switch v.taskStatusFilter {
case ecstypes.DesiredStatusStopped:
    statusLabel = "STOPPED"
case "":
    statusLabel = "ALL"
}
sb.WriteString(lipgloss.NewStyle().Foreground(colorSubtext0).Render(
    fmt.Sprintf("  Status: %s", statusLabel)))
sb.WriteString("  ")
```

"Updated ... ago"와 같은 줄에 합치거나, 별도 줄로 표시한다. 별도 줄로 표시할 경우 `rebuildTable`에서 `tableHeight` 계산 시 1줄 추가 차감이 필요하다.

**필요한 import 추가 (`internal/ui/tasks.go`):**
```go
ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
```

### 테스트 방안

- 실제 AWS 환경에서 RUNNING 태스크가 있는 서비스로 진입 후 `t` 키로 토글 확인
- STOPPED 모드에서 최근 중단된 태스크가 표시되는지 확인
- ALL 모드에서 RUNNING + STOPPED 모두 표시되는지 확인
- 필터 상태 라벨이 올바르게 변경되는지 확인
- STOPPED 태스크에서 `e` (exec) 시 기존 에러 메시지 정상 동작 확인 (245행의 `task.Status != "RUNNING"` 체크)

---

## Feature 2: Config 파일 지원

### 개요

`~/.config/ecs-tui/config.yml` 파일을 통해 기본 설정을 지정할 수 있게 한다. CLI 플래그가 config 파일 값을 오버라이드한다.

### 새 의존성

```
gopkg.in/yaml.v3
```

`go get gopkg.in/yaml.v3` 실행 필요.

### 수정/생성 파일 목록

| 파일 | 변경 유형 | 설명 |
|------|----------|------|
| `internal/config/config.go` | **새로 생성** | Config 구조체 정의, YAML 로딩, CLI 머지 로직 |
| `main.go` | 수정 | Config 로딩 후 CLI 플래그 오버라이드 적용 |
| `go.mod` / `go.sum` | 자동 수정 | `gopkg.in/yaml.v3` 의존성 추가 |

### 상세 변경사항

#### 2-1. `internal/config/config.go` - 새 파일 생성

```go
package config

import (
    "os"
    "path/filepath"

    "gopkg.in/yaml.v3"
)

// Config는 ~/.config/ecs-tui/config.yml의 설정을 담는 구조체이다.
type Config struct {
    DefaultCluster  string `yaml:"default_cluster"`
    DefaultService  string `yaml:"default_service"`
    RefreshInterval int    `yaml:"refresh_interval"` // 초 단위, -1이면 자동 새로고침 비활성화
    ReadOnly        bool   `yaml:"read_only"`
    Theme           string `yaml:"theme"`            // "mocha" 또는 "latte"
    Shell           string `yaml:"shell"`            // exec시 사용할 셸 (기본: /bin/sh)
}

// DefaultConfig는 기본 설정값을 반환한다.
func DefaultConfig() *Config {
    return &Config{
        RefreshInterval: 10,
        Theme:           "mocha",
        Shell:           "/bin/sh",
    }
}

// configFilePath는 설정 파일 경로를 반환한다.
func configFilePath() string {
    home, err := os.UserHomeDir()
    if err != nil {
        return ""
    }
    return filepath.Join(home, ".config", "ecs-tui", "config.yml")
}

// Load는 config.yml을 읽어 Config 구조체를 반환한다.
// 파일이 없으면 기본값을 반환한다.
func Load() *Config {
    cfg := DefaultConfig()
    path := configFilePath()
    if path == "" {
        return cfg
    }

    data, err := os.ReadFile(path)
    if err != nil {
        return cfg // 파일 없으면 기본값 사용
    }

    if err := yaml.Unmarshal(data, cfg); err != nil {
        return DefaultConfig() // 파싱 실패 시 기본값
    }

    return cfg
}
```

**참고:** 설정 파일 경로가 `~/.config/ecs-tui/`인 것은 기존 `session.go`의 `sessionFilePath()` (18~23행)과 동일한 디렉터리이다. 세션 파일(`session.json`)과 설정 파일(`config.yml`)이 같은 디렉터리에 위치하게 된다.

#### 2-2. `main.go` - Config 로딩 및 CLI 오버라이드

**현재 코드 (17~24행):**
```go
func main() {
    showVersion := flag.Bool("version", false, "Print version and exit")
    profile := flag.String("profile", "", "AWS profile name")
    region := flag.String("region", "", "AWS region")
    cluster := flag.String("cluster", "", "ECS cluster name")
    service := flag.String("service", "", "ECS service name (requires --cluster)")
    readOnly := flag.Bool("read-only", false, "Read-only mode (disable all mutative actions)")
    flag.Parse()
```

**변경 후:**
```go
import (
    // 기존 imports에 추가
    cfgpkg "github.com/soominna/ecs-tui/internal/config"
)

func main() {
    showVersion := flag.Bool("version", false, "Print version and exit")
    profile := flag.String("profile", "", "AWS profile name")
    region := flag.String("region", "", "AWS region")
    cluster := flag.String("cluster", "", "ECS cluster name")
    service := flag.String("service", "", "ECS service name (requires --cluster)")
    readOnly := flag.Bool("read-only", false, "Read-only mode (disable all mutative actions)")
    refreshInterval := flag.Int("refresh", 0, "Auto-refresh interval in seconds (-1 to disable)")
    flag.Parse()

    // 1. Config 파일 로드
    cfg := cfgpkg.Load()

    // 2. CLI 플래그가 명시적으로 지정된 경우 config 값을 오버라이드
    // flag.Visit를 사용하여 명시적으로 전달된 플래그만 감지
    flagSet := make(map[string]bool)
    flag.Visit(func(f *flag.Flag) {
        flagSet[f.Name] = true
    })

    if !flagSet["cluster"] && cfg.DefaultCluster != "" {
        *cluster = cfg.DefaultCluster
    }
    if !flagSet["service"] && cfg.DefaultService != "" {
        *service = cfg.DefaultService
    }
    if !flagSet["read-only"] && cfg.ReadOnly {
        *readOnly = true
    }
    if !flagSet["refresh"] {
        *refreshInterval = cfg.RefreshInterval
    }

    // 테마 적용 (CLI 플래그 없으므로 config에서만)
    if cfg.Theme != "" {
        ui.ApplyTheme(cfg.Theme)
    }
```

**핵심 사항:**
- `flag.Visit()`를 사용하여 사용자가 실제로 커맨드라인에서 지정한 플래그만 감지한다. 이를 통해 CLI 플래그가 config 파일 값을 정확히 오버라이드할 수 있다.
- `*refreshInterval` 값은 `App`에 전달하여 Feature 3에서 활용한다.
- `cfg.Shell` 값은 `ExecContainer`에 전달할 수 있도록 별도 경로가 필요하다 (아래 참조).

#### 2-3. Shell 설정을 exec에 전달하는 경로

현재 `internal/exec/exec.go`의 61행에서 `/bin/sh`가 하드코딩되어 있다:
```go
"--command", "/bin/sh",
```

**방안 A (권장): ExecContainer 함수에 shell 파라미터 추가**

`exec.go` 23행:
```go
func ExecContainer(profile, region, cluster, service, taskID, container, shell string) tea.Cmd {
```

61행:
```go
"--command", shell,
```

호출부 (`tasks.go` 250~257행) 수정:
```go
return v, execpkg.ExecContainer(
    v.client.Profile,
    v.client.Region,
    v.cluster,
    v.service,
    task.TaskID,
    task.ContainerName,
    v.shell, // TaskView에 shell 필드 추가 필요
)
```

이를 위해 `NewTaskView`에 `shell` 파라미터를 추가하거나, `App`을 통해 config를 전달해야 한다. 구체적인 전달 경로는 Feature 3의 `refreshInterval` 전달과 함께 설계한다.

### Config 파일 예시 (`~/.config/ecs-tui/config.yml`)

```yaml
# ECS-TUI 설정
default_cluster: my-production-cluster
default_service: api-service
refresh_interval: 15   # 초 단위, -1이면 자동 새로고침 비활성화
read_only: false
theme: mocha            # mocha (다크) 또는 latte (라이트)
shell: /bin/bash        # exec 시 사용할 셸
```

### 테스트 방안

- config 파일 없는 상태에서 기본값으로 정상 동작 확인
- config 파일에 `default_cluster` 설정 후 해당 클러스터로 자동 진입 확인
- CLI `--cluster` 플래그가 config의 `default_cluster`를 오버라이드하는지 확인
- 잘못된 YAML 파일에서 기본값으로 폴백하는지 확인
- `shell` 설정이 exec에 전달되는지 확인

---

## Feature 3: 자동 새로고침 간격 설정

### 개요

현재 `services.go`와 `tasks.go`의 `tickCmd()`에 10초가 하드코딩되어 있다. 이를 config 파일과 CLI 플래그(`--refresh`)를 통해 설정 가능하게 한다. `-1`이면 자동 새로고침을 완전히 비활성화한다.

### 수정 파일 목록

| 파일 | 변경 유형 | 설명 |
|------|----------|------|
| `internal/ui/app.go` | 수정 | App 구조체에 config/refreshInterval 필드 추가 |
| `internal/ui/services.go` | 수정 | 하드코딩된 10초를 동적 값으로 변경 |
| `internal/ui/tasks.go` | 수정 | 하드코딩된 10초를 동적 값으로 변경 |
| `main.go` | 수정 | refreshInterval을 App에 전달 |

### 상세 변경사항

#### 3-1. `internal/ui/app.go` - App 구조체에 설정 전달

**App 구조체 수정 (18~28행):**
```go
type App struct {
    stack    []View
    client   *awsclient.Client
    cluster  string
    service  string
    width    int
    height   int
    err      error
    status   string
    showHelp bool
    // Config
    refreshInterval time.Duration // 0이면 기본값(10s), 음수면 비활성화
    shell           string        // exec 시 사용할 셸
    readOnly        bool          // read-only 모드
}
```

**NewApp 함수 수정 (30~36행):**
```go
func NewApp(client *awsclient.Client, cluster, service string, refreshInterval int, shell string, readOnly bool) *App {
    var interval time.Duration
    if refreshInterval < 0 {
        interval = -1 // 비활성화 마커
    } else if refreshInterval == 0 {
        interval = 10 * time.Second // 기본값
    } else {
        interval = time.Duration(refreshInterval) * time.Second
    }

    if shell == "" {
        shell = "/bin/sh"
    }

    return &App{
        client:          client,
        cluster:         cluster,
        service:         service,
        refreshInterval: interval,
        shell:           shell,
        readOnly:        readOnly,
    }
}
```

**Init 함수에서 View 생성 시 interval 전달 (38~60행):**
```go
func (a *App) Init() tea.Cmd {
    if a.client == nil {
        configView := NewConfigView()
        a.stack = []View{configView}
        return configView.Init()
    }

    if a.cluster == "" {
        view := NewClusterView(a.client)
        a.stack = []View{view}
        return view.Init()
    }

    if a.service != "" {
        view := NewTaskView(a.client, a.cluster, a.service, a.refreshInterval, a.shell)
        a.stack = []View{view}
        return view.Init()
    }

    view := NewServiceView(a.client, a.cluster, a.refreshInterval)
    a.stack = []View{view}
    return view.Init()
}
```

**ClusterSelectedMsg 핸들러에서도 interval 전달 (224~229행):**
```go
case ClusterSelectedMsg:
    a.cluster = msg.ClusterName
    serviceView := NewServiceView(a.client, a.cluster, a.refreshInterval)
    return a, func() tea.Msg {
        return PushViewMsg{View: serviceView}
    }
```

**ServiceView에서 TaskView 생성하는 곳도 수정이 필요하다 (services.go 371~376행).** ServiceView가 interval과 shell을 알아야 하므로 ServiceView에도 해당 값을 전달한다.

#### 3-2. `internal/ui/services.go` - 동적 새로고침 간격

**ServiceView 구조체에 필드 추가 (19행 부근):**
```go
type ServiceView struct {
    // ... 기존 필드 ...
    refreshInterval time.Duration
    shell           string // TaskView 생성 시 전달용
}
```

**NewServiceView 수정 (62~79행):**
```go
func NewServiceView(client *awsclient.Client, cluster string, refreshInterval time.Duration) *ServiceView {
    // ... 기존 코드 ...
    return &ServiceView{
        client:          client,
        cluster:         cluster,
        taskDefs:        make(map[string]*awsclient.TaskDefinitionInfo),
        metrics:         make(map[string]*awsclient.ServiceMetrics),
        filterInput:     ti,
        countInput:      ci,
        refreshInterval: refreshInterval,
    }
}
```

**tickCmd 수정 (117~121행):**

현재:
```go
func (v *ServiceView) tickCmd() tea.Cmd {
    return tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
        return serviceTickMsg(t)
    })
}
```

변경:
```go
func (v *ServiceView) tickCmd() tea.Cmd {
    // 음수면 자동 새로고침 비활성화
    if v.refreshInterval < 0 {
        return nil
    }
    interval := v.refreshInterval
    if interval == 0 {
        interval = 10 * time.Second
    }
    return tea.Tick(interval, func(t time.Time) tea.Msg {
        return serviceTickMsg(t)
    })
}
```

**ServiceView에서 TaskView 생성 시 interval 전달 (370~377행):**
```go
case "enter":
    svc := v.selectedService()
    if svc != nil {
        taskView := NewTaskView(v.client, v.cluster, svc.Name, v.refreshInterval, v.shell)
        return v, func() tea.Msg {
            return PushViewMsg{View: taskView}
        }
    }
```

#### 3-3. `internal/ui/tasks.go` - 동적 새로고침 간격

**TaskView 구조체에 필드 추가 (18행 부근):**
```go
type TaskView struct {
    // ... 기존 필드 ...
    refreshInterval time.Duration
    shell           string // exec 시 사용할 셸
}
```

**NewTaskView 수정 (47~58행):**
```go
func NewTaskView(client *awsclient.Client, cluster, service string, refreshInterval time.Duration, shell string) *TaskView {
    ti := textinput.New()
    ti.Placeholder = "Filter tasks..."
    ti.CharLimit = 50

    return &TaskView{
        client:           client,
        cluster:          cluster,
        service:          service,
        filterInput:      ti,
        taskStatusFilter: ecstypes.DesiredStatusRunning,
        refreshInterval:  refreshInterval,
        shell:            shell,
    }
}
```

**tickCmd 수정 (90~94행):**

현재:
```go
func (v *TaskView) tickCmd() tea.Cmd {
    return tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
        return taskTickMsg(t)
    })
}
```

변경:
```go
func (v *TaskView) tickCmd() tea.Cmd {
    if v.refreshInterval < 0 {
        return nil
    }
    interval := v.refreshInterval
    if interval == 0 {
        interval = 10 * time.Second
    }
    return tea.Tick(interval, func(t time.Time) tea.Msg {
        return taskTickMsg(t)
    })
}
```

#### 3-4. `main.go` - NewApp 호출 수정

**현재 코드 (65행):**
```go
app := ui.NewApp(client, *cluster, *service)
```

**변경 후:**
```go
app := ui.NewApp(client, *cluster, *service, *refreshInterval, cfg.Shell, *readOnly)
```

#### 3-5. awsClientReadyMsg 핸들러 (app.go 203~217행)

AWS 프로필 변경 후 다시 ClusterView를 만들 때도 interval이 유지되어야 한다:
```go
case awsClientReadyMsg:
    // ... 기존 코드 동일 ...
    clusterView := NewClusterView(a.client)
    a.stack = []View{clusterView}
    return a, clusterView.Init()
```
ClusterView는 자동 새로고침이 없으므로 변경 불필요. ServiceView/TaskView는 이후 네비게이션에서 `a.refreshInterval`을 참조하여 생성되므로 정상 동작한다.

### 테스트 방안

- `--refresh 5`로 실행 후 5초 간격으로 갱신되는지 확인
- `--refresh -1`로 실행 후 자동 갱신이 발생하지 않는 것 확인 (수동 `r` 키는 여전히 동작해야 함)
- config 파일에 `refresh_interval: 30` 설정 후 30초 간격 확인
- CLI `--refresh 5`가 config의 `refresh_interval: 30`을 오버라이드하는지 확인
- 기본값(플래그 미지정, config 없음) 상태에서 10초 간격 유지 확인

---

## 구현 순서 권장사항

### 권장 순서: Feature 2 -> Feature 3 -> Feature 1

**이유:**
1. **Feature 2 (Config 파일)** 를 먼저 구현하면 `Config` 구조체와 로딩 인프라가 확보된다.
2. **Feature 3 (새로고침 간격)** 은 Feature 2의 `refresh_interval` 설정과 `shell` 전달 경로를 활용한다. `App` 구조체를 통한 설정 전달 패턴이 여기서 확립된다.
3. **Feature 1 (Stopped Tasks)** 은 독립적이므로 마지막에 추가해도 충돌이 없다.

### 의존성 그래프

```
Feature 2 (Config)
    └──> Feature 3 (Refresh Interval) - config.refresh_interval 사용
    └──> Feature 1 (Stopped Tasks) - 독립적, 어떤 순서든 가능

main.go 수정은 3개 Feature가 모두 반영된 최종 버전으로 한 번에 작성하는 것이 효율적이다.
```

### 최종 main.go 모습 (3개 Feature 모두 적용)

```go
func main() {
    showVersion := flag.Bool("version", false, "Print version and exit")
    profile := flag.String("profile", "", "AWS profile name")
    region := flag.String("region", "", "AWS region")
    cluster := flag.String("cluster", "", "ECS cluster name")
    service := flag.String("service", "", "ECS service name (requires --cluster)")
    readOnly := flag.Bool("read-only", false, "Read-only mode (disable all mutative actions)")
    refreshInterval := flag.Int("refresh", 0, "Auto-refresh interval in seconds (-1 to disable)")
    flag.Parse()

    if *showVersion {
        fmt.Printf("ecs-tui version %s\n", version)
        os.Exit(0)
    }

    if *service != "" && *cluster == "" {
        fmt.Fprintln(os.Stderr, "Error: --service requires --cluster")
        os.Exit(1)
    }

    // Config 파일 로드
    cfg := cfgpkg.Load()

    // CLI 플래그 오버라이드
    flagSet := make(map[string]bool)
    flag.Visit(func(f *flag.Flag) { flagSet[f.Name] = true })

    if !flagSet["cluster"] && cfg.DefaultCluster != "" {
        *cluster = cfg.DefaultCluster
    }
    if !flagSet["service"] && cfg.DefaultService != "" {
        *service = cfg.DefaultService
    }
    if !flagSet["read-only"] && cfg.ReadOnly {
        *readOnly = true
    }
    if !flagSet["refresh"] {
        *refreshInterval = cfg.RefreshInterval
    }

    // 테마 적용
    if cfg.Theme != "" {
        ui.ApplyTheme(cfg.Theme)
    }

    // ... (기존 세션 복원 및 클라이언트 생성 로직 유지) ...

    app := ui.NewApp(client, *cluster, *service, *refreshInterval, cfg.Shell, *readOnly)

    p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())
    if _, err := p.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

---

## 변경 파일 요약

| # | 파일 | Feature 1 | Feature 2 | Feature 3 |
|---|------|-----------|-----------|-----------|
| 1 | `internal/config/config.go` | - | **새로 생성** | - |
| 2 | `internal/aws/ecs.go` | **수정** (ListTasks 시그니처 + ListTasksAll 추가) | - | - |
| 3 | `internal/ui/app.go` | - | - | **수정** (구조체 필드, NewApp 시그니처) |
| 4 | `internal/ui/tasks.go` | **수정** (필터 토글, 키 핸들러, View 표시) | - | **수정** (tickCmd 동적 간격) |
| 5 | `internal/ui/services.go` | - | - | **수정** (tickCmd 동적 간격, 구조체 필드) |
| 6 | `internal/exec/exec.go` | - | **수정** (shell 파라미터) | - |
| 7 | `main.go` | - | **수정** (config 로딩, 플래그 오버라이드) | **수정** (refresh 플래그, NewApp 호출) |
| 8 | `go.mod` / `go.sum` | - | **수정** (yaml.v3 추가) | - |

총 수정 파일: **7개** (신규 1개 포함) + go.mod/go.sum 자동 변경
