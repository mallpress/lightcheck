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

func AddHealthCheckHandler(server *grpc.Server, checks []HealthCheckService) {
	checkList := make([]*HealthCheckService, 0)
	for _, c := range checks {
		cc := c
		checkList = append(checkList, &cc)
	}

	checker := healthChecker{checkList, make([]*checkResponse, 0), sync.Mutex{}}

	RegisterLightCheckServer(server, checker)
}

type healthChecker struct {
	checkers		[]*HealthCheckService
	responses 		[]*checkResponse
	runningMutex 	sync.Mutex
}

type HealthCheckService struct {
	Name      string
	CheckFunc func() (*ServiceDependancy, error)
	Required  bool
}

type checkResponse struct {
	Check    *HealthCheckService
	Response *ServiceDependancy
	Error    error
}

func (h healthChecker) HealthCheck(context.Context, *HealthCheckRequest) (*HealthCheckResponse, error) {
	h.runningMutex.Lock()
	h.responses = make([]*checkResponse, 0)
	checkerCount := len(h.checkers)

	resp := HealthCheckResponse{}
	status := ServiceStatus_UP
	resp.Status = &status
	msg := "Check all the healths"
	resp.Message = &msg
	respChan := make(chan checkResponse, checkerCount)

	defer h.runningMutex.Unlock()

	for _, f := range h.checkers {
		tempCheck := *f
		go func() {
			done := false
			doneMutex := sync.Mutex{}
			go func() {
				res, err := tempCheck.CheckFunc()
				doneMutex.Lock()
				if !done {
					respChan <- checkResponse{&tempCheck, res, err}
					done = true
				}
				doneMutex.Unlock()
			}()
			select {
			case <-time.After(HEALTHCHECK_SERVICE_TIMEOUT):
				doneMutex.Lock()
				if !done {
					msg := "WE TIMED OUT"
					downStatus := ServiceStatus_DOWN
					respChan <- checkResponse{&tempCheck, &ServiceDependancy{&tempCheck.Name, &msg, &downStatus, nil}, fmt.Errorf("Timed out waiting for something")}
					done = true
				}
				doneMutex.Unlock()
			}
		}()
	}

	responses := make([]*ServiceDependancy, 0)

	for i := 0; i < checkerCount; i++ {
		select {
		case res := <-respChan:
			responses = append(responses, res.Response)
			if res.Check.Required && (*res.Response.Status == ServiceStatus_DOWN || *res.Response.Status == ServiceStatus_DEGRADED) {
				status = ServiceStatus_DEGRADED
			}
		case <-time.After(HEALTHCHECK_TIMEOUT):
			status = ServiceStatus_DEGRADED
			break
		}
	}
	resp.Dependancies = responses
	return &resp, nil
}
