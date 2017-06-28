package lightcheck

import (
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"sync"
	"time"
)

var HEALTHCHECK_TIMEOUT time.Duration = 3 * time.Second
var HEALTHCHECK_SERVICE_TIMEOUT time.Duration = 1 * time.Second

func AddHealthCheckHandler(server *grpc.Server, config LightCheckConfig) {
	checkList := make([]*HealthCheckService, 0)
	var primaryCheck *HealthCheckService
	if config.PrimaryCheckFunc != nil {
		primaryCheck = &HealthCheckService{config.Name, config.PrimaryCheckFunc, true}
		checkList = append(checkList, primaryCheck)
	}
	for _, c := range config.DependencyChecks {
		cc := c
		checkList = append(checkList, &cc)
	}

	checker := healthChecker{&config, primaryCheck, checkList, make([]*checkResponse, 0), sync.Mutex{}}

	RegisterLightCheckServer(server, checker)
}

type healthChecker struct {
	config       *LightCheckConfig
	primaryCheck *HealthCheckService
	checkers     []*HealthCheckService
	responses    []*checkResponse
	runningMutex sync.Mutex
}

type LightCheckConfig struct {
	Name             string
	PrimaryCheckFunc func() (*ServiceDependency, error)
	DependencyChecks []HealthCheckService
}

type HealthCheckService struct {
	Name      string
	CheckFunc func() (*ServiceDependency, error)
	Required  bool
}

type checkResponse struct {
	Check    *HealthCheckService
	Response *ServiceDependency
	Error    error
}

func (h healthChecker) HealthCheck(context.Context, *HealthCheckRequest) (*HealthCheckResponse, error) {
	h.runningMutex.Lock()
	h.responses = make([]*checkResponse, 0)
	checkerCount := len(h.checkers)

	resp := HealthCheckResponse{}
	status := ServiceStatus_UP
	resp.Status = status
	msg := ""
	respChan := make(chan checkResponse, checkerCount)

	defer h.runningMutex.Unlock()

	for _, f := range h.checkers {
		tempCheck := f
		go func() {
			done := false
			doneMutex := sync.Mutex{}
			go func() {
				res, err := tempCheck.CheckFunc()
				doneMutex.Lock()
				if !done {
					respChan <- checkResponse{tempCheck, res, err}
					done = true
				}
				doneMutex.Unlock()
			}()
			select {
			case <-time.After(HEALTHCHECK_SERVICE_TIMEOUT):
				doneMutex.Lock()
				if !done {
					msg := "Serivce timed out"
					downStatus := ServiceStatus_DOWN
					respChan <- checkResponse{tempCheck, &ServiceDependency{tempCheck.Name, msg, downStatus, "", ""}, fmt.Errorf("Timed out waiting for something")}
					done = true
				}
				doneMutex.Unlock()
			}
		}()
	}

	responses := make([]*ServiceDependency, 0)

	for i := 0; i < checkerCount; i++ {
		select {
		case res := <-respChan:
			if res.Check == h.primaryCheck {
				status = res.Response.Status
				msg = res.Response.Message
			} else {
				responses = append(responses, res.Response)
				if res.Check.Required && status == ServiceStatus_UP && (res.Response.Status == ServiceStatus_DOWN || res.Response.Status == ServiceStatus_DEGRADED) {
					status = ServiceStatus_DEGRADED
				}
			}
		case <-time.After(HEALTHCHECK_TIMEOUT):
			status = ServiceStatus_DEGRADED
			msg = "Service checks timed out!"
			break
		}
	}
	resp.Message = msg
	resp.Status = status
	resp.Dependencies = responses
	return &resp, nil
}
