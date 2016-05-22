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
	checker := HealthChecker{checks, make(map[string]*checkResponse)}

	RegisterLightCheckServer(server, checker)
}

type HealthChecker struct {
	checkers  []HealthCheckService
	responses map[string]*checkResponse
}

type HealthCheckService struct {
	Name      string
	CheckFunc func() (*ServiceDependancy, error)
}

type checkResponse struct {
	Name     string
	Response *ServiceDependancy
	Error    error
}

func (h HealthChecker) HealthCheck(context.Context, *HealthCheckRequest) (*HealthCheckResponse, error) {

	checkerCount := len(h.checkers)

	resp := HealthCheckResponse{}
	status := ServiceStatus_UP
	resp.Status = &status
	msg := "Check all the healths"
	resp.Message = &msg
	respChan := make(chan checkResponse, checkerCount)

	for _, f := range h.checkers {
		tempCheck := f
		go func() {
			done := false
			doneMutex := sync.Mutex{}
			go func() {
				res, err := tempCheck.CheckFunc()
				doneMutex.Lock()
				if !done {
					respChan <- checkResponse{tempCheck.Name, res, err}
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
					respChan <- checkResponse{tempCheck.Name, &ServiceDependancy{&tempCheck.Name, &msg, &downStatus, nil}, fmt.Errorf("Timed out waiting for something")}
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
			if *res.Response.Status == ServiceStatus_DOWN || *res.Response.Status == ServiceStatus_DEGRADED {
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
