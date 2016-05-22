package main

import (
	"../../lightcheck"
	"google.golang.org/grpc"
	"log"
	"net"
	"time"
)

const (
	port = ":9001"
)

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()

	svc0 := lightcheck.HealthCheckService{"Something to timeout", func() (*lightcheck.ServiceDependancy, error) {
		time.Sleep(5 * time.Second)
		return nil, nil
	}, false}

	toUse := make([]lightcheck.HealthCheckService, 0)
	toUse = append(toUse, svc0)

	svc1 := lightcheck.HealthCheckService{"Something that works", func() (*lightcheck.ServiceDependancy, error) {
		msg := "ITS WORKING"
		name := "WORK"
		status := lightcheck.ServiceStatus_UP
		dp := lightcheck.ServiceDependancy{&name, &msg, &status, nil}
		return &dp, nil
	}, true}

	toUse = append(toUse, svc1)

	lightcheck.AddHealthCheckHandler(s, toUse)
	s.Serve(lis)
}
