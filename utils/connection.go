package utils

import (
	"log"

	"github.com/fbsobreira/gotron-sdk/pkg/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	DefaultNode = "grpc.trongrid.io:50051"
)

// NewGRPCClient creates a new gotron-sdk gRPC client connected to the given node.
// If node is empty, it defaults to DefaultNode.
//
// Note: TronGrid's public gRPC endpoint (port 50051) does not use TLS, so insecure
// credentials are required. Do not use this pattern for nodes that support TLS.
func NewGRPCClient(node string) *client.GrpcClient {
	if node == "" {
		node = DefaultNode
	}
	conn := client.NewGrpcClient(node)
	if err := conn.Start(grpc.WithTransportCredentials(insecure.NewCredentials())); err != nil {
		log.Fatalf("failed to connect to %s: %v", node, err)
	}
	apiKey := GetAPIKey()
	if apiKey != "" {
		if err := conn.SetAPIKey(apiKey); err != nil {
			log.Fatalf("failed to set API key: %v", err)
		}
	}
	return conn
}
