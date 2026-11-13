// cft-kv runs one Paxi Paxos replica whose executor applies committed
// commands to the Altun replicated KV state machine.  Clients reach
// it only through the Altun proxy (POST / with a JSON cft.Command whose Value
// is an altunsm.Op).  Run with -ephemeral_leader so that the replica the proxy
// talks to runs phase 1 itself after a leader failure.
package main

import (
	"flag"

	"github.com/wtwinlab/altun/consensus/cft"
	"github.com/wtwinlab/altun/consensus/cft/paxos"
	"github.com/wtwinlab/altun/lib/log"
	"github.com/wtwinlab/altunsm"
)

var id = flag.String("id", "", "ID in format of Zone.Node.")
var nodigest = flag.Bool("unsafe-nodigest", false, "skip the segstart digest check (fault-injection runs only)")
var byz = flag.String("byz", "", "byzantine state-machine behaviour: corrupt | lie | lie-ok | fork (tests only)")
var attKeys = flag.String("attkeys", "", "directory with NAME.key / *.pub ed25519 attestation keys (CERTIFY)")
var attName = flag.String("name", "", "this replica's attestation key name, e.g. paxos1")
var requireCert = flag.Bool("require-cert", false, "CERTIFY: install a snapshot only with a valid checkpoint certificate in segstart")

func main() {
	cft.Init()
	cft.App = altunsm.New()
	cft.App.NoDigestCheck = *nodigest
	cft.App.Byz = *byz
	cft.App.RequireCert = *requireCert
	if *attKeys != "" {
		if err := cft.App.UseKeys(*attKeys, *attName); err != nil {
			log.Fatalf("attestation keys: %v", err)
		}
	}
	log.Infof("cft-kv node %v starting", *id)
	paxos.NewReplica(cft.ID(*id)).Run()
}
