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

	svc0 := lightcheck.HealthCheckService{"Something to timeout", func() (*lightcheck.ServiceDependency, error) {
		time.Sleep(5 * time.Second)
		return nil, nil
	}, true}

	toUse := make([]lightcheck.HealthCheckService, 0)
	toUse = append(toUse, svc0)

	svc1 := lightcheck.HealthCheckService{"Something that works", func() (*lightcheck.ServiceDependency, error) {
		msg := "ITS WORKING"
		name := "WORK"
		status := lightcheck.ServiceStatus_UP
		dp := lightcheck.ServiceDependency{&name, &msg, &status, nil}
		return &dp, nil
	}, true}

	toUse = append(toUse, svc1)

	config := lightcheck.LightCheckConfig{"THIS IS A TEST CHECK THING", func() (*lightcheck.ServiceDependency, error) {
		status := lightcheck.ServiceStatus_DOWN
		msg := "All quiet on the western front"
		name := "Main process"
		dp := lightcheck.ServiceDependency{&name, &msg, &status, nil}
		return &dp, nil
	}, toUse}

	lightcheck.AddHealthCheckHandler(s, config)
	s.Serve(lis)
}
