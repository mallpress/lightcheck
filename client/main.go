package main

import (
	"../../lightcheck"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"log"
)

const (
	address = "localhost:9001"
)

func main() {
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := lightcheck.NewLightCheckClient(conn)
	rq := lightcheck.HealthCheckRequest{}
	resp, err := c.HealthCheck(context.Background(), &rq)
	fmt.Printf("%+v", resp)
}
