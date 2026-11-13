package main

import (
	"encoding/binary"
	"flag"
	"os"
	"strconv"

	"github.com/wtwinlab/altun/benchmark"
	"github.com/wtwinlab/altun/consensus/cft"
	"github.com/wtwinlab/altun/consensus/cft/paxos"
)

var id = flag.String("id", "", "node id this client connects to")
var algorithm = flag.String("algorithm", "", "Client API type [paxos]")
var load = flag.Bool("load", false, "Load K keys into DB")
var master = flag.String("master", "", "Master address.")

// db implements cft.DB interface for benchmarking
type bench struct {
	cft.Client
}

func (d *bench) Init() error {
	return nil
}

func (d *bench) Stop() error {
	return nil
}

func (d *bench) Read(k int) (int, error) {
	key := cft.Key(k)
	v, err := d.Get(key)
	if len(v) == 0 {
		return 0, nil
	}
	x, _ := binary.Uvarint(v)
	return int(x), err
}

// payloadSize is the request value size in bytes. apply-cft.sh exports
// CFT_PAYLOAD_SIZE=<payloadsize> when it launches the client; unset or smaller
// than a varint keeps the minimal 10-byte value.
var payloadSize = func() int {
	n, _ := strconv.Atoi(os.Getenv("CFT_PAYLOAD_SIZE"))
	if n < binary.MaxVarintLen64 {
		n = binary.MaxVarintLen64
	}
	return n
}()

func (d *bench) Write(k, v int) error {
	key := cft.Key(k)
	value := make([]byte, payloadSize)
	binary.PutUvarint(value, uint64(v)) // Read decodes this prefix; the rest is payload
	err := d.Put(key, value)
	return err
}

func main() {
	cft.Init()

	if *master != "" {
		cft.ConnectToMaster(*master, true, cft.ID(*id))
	}

	d := new(bench)
	switch *algorithm {
	case "paxos":
		d.Client = paxos.NewClient(cft.ID(*id))
	default:
		d.Client = cft.NewHTTPClient(cft.ID(*id))
	}

	b := benchmark.NewBenchmark(d)
	if *load {
		b.Load()
	} else {
		b.Run()
	}
}
