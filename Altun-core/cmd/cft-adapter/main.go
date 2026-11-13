// cft-adapter wraps an in-process CFT consensus replica behind a gRPC
// ConsensusInstance service. Supports paxos, epaxos, vpaxos, wpaxos.
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/wtwinlab/altun/consensus/cft"
	"github.com/wtwinlab/altun/consensus/cft/adapter"
	"github.com/wtwinlab/altun/consensus/cft/epaxos"
	"github.com/wtwinlab/altun/consensus/cft/paxos"
	"github.com/wtwinlab/altun/consensus/cft/vpaxos"
	"github.com/wtwinlab/altun/consensus/cft/wpaxos"
	"github.com/wtwinlab/altun/proto/altunpb"
	"google.golang.org/grpc"
)

var (
	protocol   = flag.String("protocol", "paxos", "consensus protocol (paxos|epaxos|vpaxos|wpaxos)")
	grpcPort   = flag.Int("grpc-port", 9100, "gRPC listen port")
	instanceID = flag.Int64("instance-id", 1, "unique instance identifier")
	nodeID     = flag.String("id", "", "node ID in format Zone.Node")
)

func main() {
	cft.Init()

	id := cft.ID(*nodeID)
	if id == "" {
		for k := range cft.GetConfig().Addrs {
			id = k
			break
		}
	}

	// Create protocol replica and extract Node + CommitTracker
	var node cft.Node
	var tracker adapter.CommitTracker
	var runFn func() // starts the replica's consensus + HTTP loops

	switch *protocol {
	case "paxos":
		r := paxos.NewReplica(id)
		node = r.Node
		tracker = r.Paxos
		runFn = r.Run
	case "epaxos":
		r := epaxos.NewReplica(id)
		node = r.Node
		tracker = &adapter.EPaxosTracker{R: r}
		runFn = r.Run
	case "vpaxos":
		r := vpaxos.NewReplica(id)
		node = r.Node
		tracker = &adapter.VPaxosTracker{R: r}
		runFn = r.Run
	case "wpaxos":
		r := wpaxos.NewReplica(id)
		node = r.Node
		tracker = &adapter.WPaxosTracker{R: r}
		runFn = r.Run
	default:
		fmt.Fprintf(os.Stderr, "unsupported protocol: %s\n", *protocol)
		os.Exit(1)
	}

	// Start replica in background (consensus messaging + HTTP client API)
	go runFn()

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *grpcPort))
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen port %d: %v\n", *grpcPort, err)
		os.Exit(1)
	}

	srv := grpc.NewServer()
	svc := adapter.NewCFTAdapter(*instanceID, *protocol, node, tracker)
	altunpb.RegisterConsensusInstanceServer(srv, svc)

	go func() {
		fmt.Printf("cft-adapter: %s instance %d on :%d\n", *protocol, *instanceID, *grpcPort)
		if err := srv.Serve(lis); err != nil {
			fmt.Fprintf(os.Stderr, "grpc: %v\n", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh
	fmt.Println("cft-adapter: shutting down")
	srv.GracefulStop()
}
