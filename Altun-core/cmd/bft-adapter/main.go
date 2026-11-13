// bft-adapter wraps a BFT consensus subprocess (bft-server) behind a gRPC
// ConsensusInstance service. The controller manages this process's lifecycle.
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	bftadapter "github.com/wtwinlab/altun/consensus/bft/adapter"
	"github.com/wtwinlab/altun/proto/altunpb"
	"google.golang.org/grpc"
)

var (
	protocol   = flag.String("protocol", "chainedhotstuff", "BFT protocol")
	grpcPort   = flag.Int("grpc-port", 9200, "gRPC listen port")
	instanceID = flag.Int64("instance-id", 1, "unique instance identifier")
	bftBin     = flag.String("bft-bin", "./bft-server", "path to bft-server binary")
	replicas   = flag.Int("replicas", 4, "number of replicas")
	clients    = flag.Int("clients", 1, "number of clients")
	batchSize  = flag.Int("batch-size", 1, "batch size")
	payload    = flag.Int("payload-size", 128, "payload size in bytes")
	rateLimit  = flag.Int("rate-limit", 10000, "rate limit (commands/sec)")
	duration   = flag.Int("duration", 30, "benchmark duration in seconds")
	outputDir  = flag.String("output", "/tmp/bft-output", "measurement output dir")
)

func main() {
	flag.Parse()

	cfg := bftadapter.BFTConfig{
		BinPath:     *bftBin,
		Protocol:    *protocol,
		Replicas:    *replicas,
		Clients:     *clients,
		BatchSize:   *batchSize,
		PayloadSize: *payload,
		RateLimit:   *rateLimit,
		Duration:    *duration,
		OutputDir:   *outputDir,
	}

	svc := bftadapter.NewBFTAdapter(*instanceID, cfg)
	if err := svc.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start bft-server: %v\n", err)
		os.Exit(1)
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *grpcPort))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to listen on port %d: %v\n", *grpcPort, err)
		os.Exit(1)
	}

	srv := grpc.NewServer()
	altunpb.RegisterConsensusInstanceServer(srv, svc)

	go func() {
		fmt.Printf("bft-adapter: %s instance %d listening on :%d\n", *protocol, *instanceID, *grpcPort)
		if err := srv.Serve(lis); err != nil {
			fmt.Fprintf(os.Stderr, "grpc serve error: %v\n", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh
	fmt.Println("bft-adapter: shutting down")
	srv.GracefulStop()
}
