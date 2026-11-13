package main

import (
	"flag"
	"sync"

	"github.com/wtwinlab/altun/consensus/cft"
	"github.com/wtwinlab/altun/consensus/cft/auditor"

	"github.com/wtwinlab/altun/lib/log"

	"github.com/wtwinlab/altun/consensus/cft/epaxos"
	"github.com/wtwinlab/altun/consensus/cft/paxos"
	"github.com/wtwinlab/altun/consensus/cft/vpaxos"
	"github.com/wtwinlab/altun/consensus/cft/wpaxos"
)

var algorithm = flag.String("algorithm", "paxos", "Distributed algorithm")
var id = flag.String("id", "", "ID in format of Zone.Node.")
var simulation = flag.Bool("sim", false, "simulation mode")

var master = flag.String("master", "", "Master address.")

func replica(id cft.ID) {
	if *master != "" {
		cft.ConnectToMaster(*master, false, id)
	}

	log.Infof("node %v starting...", id)

	switch *algorithm {

	case "paxos":
		paxos.NewReplica(id).Run()
	case "epaxos":
		epaxos.NewReplica(id).Run()
	case "vpaxos":
		vpaxos.NewReplica(id).Run()
	case "wpaxos":
		wpaxos.NewReplica(id).Run()

	default:
		panic("Unknown algorithm")
	}
}

func main() {
	cft.Init()     // load config
	auditor.Init() // activate equivocation auditor if AUDITOR_ENABLED=1

	if *simulation {
		var wg sync.WaitGroup
		wg.Add(1)
		cft.Simulation()
		for id := range cft.GetConfig().Addrs {
			n := id
			go replica(n)
		}
		wg.Wait()
	} else {
		replica(cft.ID(*id))
	}
}
