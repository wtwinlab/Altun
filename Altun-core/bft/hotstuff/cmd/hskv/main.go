// hskv runs one chained-HotStuff replica with the Altun replicated KV state
// machine and the proxy-facing HTTP front end used for online protocol switching.
//
//	hskv -genkeys -n 4 -keys DIR                  # once: ECDSA keys for replicas 1..n
//	hskv -id 1 -n 4 -keys DIR -host 127.0.0.1 -port 21000 -kvport 22000
//
// Replica i listens for replicas on port+i, for gorums clients on port+100+i
// and for the proxy on kvport+i.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/relab/gorums"
	"github.com/relab/hotstuff"
	"github.com/relab/hotstuff/backend"
	"github.com/relab/hotstuff/blockchain"
	"github.com/relab/hotstuff/consensus"
	"github.com/relab/hotstuff/crypto"
	"github.com/relab/hotstuff/crypto/keygen"
	"github.com/relab/hotstuff/eventloop"
	"github.com/relab/hotstuff/logging"
	"github.com/relab/hotstuff/modules"
	"github.com/relab/hotstuff/replica"
	"github.com/relab/hotstuff/synchronizer"
	"github.com/wtwinlab/altunsm"
	"google.golang.org/grpc"

	_ "github.com/relab/hotstuff/consensus/chainedhotstuff"
	_ "github.com/relab/hotstuff/crypto/ecdsa"
	_ "github.com/relab/hotstuff/leaderrotation"
)

func main() {
	genkeys := flag.Bool("genkeys", false, "generate keys and exit")
	id := flag.Int("id", 1, "replica id (1..n)")
	n := flag.Int("n", 4, "number of replicas")
	keys := flag.String("keys", "keys", "key directory")
	host := flag.String("host", "127.0.0.1", "host of all replicas")
	port := flag.Int("port", 21000, "replica port base")
	kvport := flag.Int("kvport", 22000, "KV front-end port base")
	batch := flag.Int("batch", 64, "max commands per block")
	idle := flag.Duration("idle", 5*time.Millisecond, "empty-proposal interval")
	viewTimeout := flag.Duration("view-timeout", 100*time.Millisecond, "initial view timeout")
	maxTimeout := flag.Duration("max-timeout", 300*time.Millisecond, "max view timeout")
	opTimeout := flag.Duration("op-timeout", 3*time.Second, "front-end op timeout")
	rotation := flag.String("leader-rotation", "round-robin", "round-robin | carousel | reputation | fixed")
	byz := flag.String("byz", "", "Byzantine state-machine behaviour for fault injection: corrupt | lie | lie-ok")
	nodigest := flag.Bool("unsafe-nodigest", false, "disable the segment-start digest check (fault-injection control)")
	attKeys := flag.String("attkeys", "", "directory with NAME.key / *.pub ed25519 attestation keys for checkpoint certification")
	attName := flag.String("name", "", "this replica's attestation key name (default hotstuff<id>)")
	requireCert := flag.Bool("require-cert", false, "install a snapshot only with a valid checkpoint certificate in the segment-start record")
	logLevel := flag.String("log-level", "info", "hotstuff log level")
	flag.Parse()
	logging.SetLogLevel(*logLevel)

	if *genkeys {
		if err := os.MkdirAll(*keys, 0o755); err != nil {
			log.Fatal(err)
		}
		for i := 1; i <= *n; i++ {
			pk, err := keygen.GenerateECDSAPrivateKey()
			if err != nil {
				log.Fatal(err)
			}
			if err := keygen.WritePrivateKeyFile(pk, filepath.Join(*keys, fmt.Sprintf("r%d.key", i))); err != nil {
				log.Fatal(err)
			}
			if err := keygen.WritePublicKeyFile(&pk.PublicKey, filepath.Join(*keys, fmt.Sprintf("r%d.pub", i))); err != nil {
				log.Fatal(err)
			}
		}
		return
	}

	privKey, err := keygen.ReadPrivateKeyFile(filepath.Join(*keys, fmt.Sprintf("r%d.key", *id)))
	if err != nil {
		log.Fatal(err)
	}
	var infos []backend.ReplicaInfo
	for i := 1; i <= *n; i++ {
		pub, err := keygen.ReadPublicKeyFile(filepath.Join(*keys, fmt.Sprintf("r%d.pub", i)))
		if err != nil {
			log.Fatal(err)
		}
		infos = append(infos, backend.ReplicaInfo{ID: hotstuff.ID(i), Address: net.JoinHostPort(*host, strconv.Itoa(*port+i)), PubKey: pub})
	}

	builder := modules.NewBuilder(hotstuff.ID(*id), privKey)
	rules, _ := modules.GetModule[consensus.Rules]("chainedhotstuff")
	cryptoImpl, _ := modules.GetModule[modules.CryptoBase]("ecdsa")
	lr, ok := modules.GetModule[modules.LeaderRotation](*rotation)
	if !ok {
		log.Fatalf("unknown leader rotation %q", *rotation)
	}
	sync := synchronizer.New(synchronizer.NewViewDuration(1000,
		float64(viewTimeout.Nanoseconds())/float64(time.Millisecond),
		float64(maxTimeout.Nanoseconds())/float64(time.Millisecond), 1.2))
	builder.Add(
		eventloop.New(1000),
		consensus.New(rules),
		consensus.NewVotingMachine(),
		crypto.NewCache(cryptoImpl, 100),
		lr,
		sync,
		blockchain.New(),
		logging.New("hs"+strconv.Itoa(*id)),
	)
	builder.Options().SetSharedRandomSeed(1)
	r := replica.New(replica.Config{
		ID:         hotstuff.ID(*id),
		PrivateKey: privKey,
		BatchSize:  uint32(*batch),
		ManagerOptions: []gorums.ManagerOption{
			gorums.WithDialTimeout(5 * time.Second),
			gorums.WithGrpcDialOptions(grpc.WithReturnConnectionError()),
		},
		LocationInfo: map[hotstuff.ID]string{},
	}, builder)

	app := altunsm.New()
	app.Byz = *byz
	app.NoDigestCheck = *nodigest
	app.RequireCert = *requireCert
	if *attKeys != "" {
		name := *attName
		if name == "" {
			name = fmt.Sprintf("hotstuff%d", *id)
		}
		if err := app.UseKeys(*attKeys, name); err != nil {
			log.Fatalf("attestation keys: %v", err)
		}
	}
	if err := r.StartKVFront(net.JoinHostPort("0.0.0.0", strconv.Itoa(*kvport+*id)), app, *idle, *opTimeout); err != nil {
		log.Fatal(err)
	}
	rl, err := net.Listen("tcp", net.JoinHostPort("0.0.0.0", strconv.Itoa(*port+*id)))
	if err != nil {
		log.Fatal(err)
	}
	cl, err := net.Listen("tcp", net.JoinHostPort("0.0.0.0", strconv.Itoa(*port+100+*id)))
	if err != nil {
		log.Fatal(err)
	}
	r.StartServers(rl, cl)

	// wait until every peer listens before the one-shot Connect
	deadline := time.Now().Add(300 * time.Second)
	for _, in := range infos {
		for {
			c, err := net.DialTimeout("tcp", in.Address, time.Second)
			if err == nil {
				c.Close()
				break
			}
			if time.Now().After(deadline) {
				log.Fatalf("peer %v not reachable", in.Address)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	if err := r.Connect(infos); err != nil {
		log.Fatal(err)
	}
	r.Start()
	log.Printf("hskv replica %d of %d running (kv :%d, byz=%q)", *id, *n, *kvport+*id, *byz)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	r.Stop()
}
