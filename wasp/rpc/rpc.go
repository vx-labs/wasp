package rpc

import (
	"google.golang.org/grpc"
)

func Listen() *grpc.Server {
	return grpc.NewServer()
}
