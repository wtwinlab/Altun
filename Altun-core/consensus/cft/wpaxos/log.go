package wpaxos

import (
	"time"

	"github.com/wtwinlab/altun/consensus/cft"
)

// TD same as instance in wpaxos/paxos
type entry struct {
	ballot    int
	cmds      []cft.Command
	committed bool
	request   *cft.Request
	quorum    *cft.Quorum
	timestamp time.Time
}

type index struct {
	i, j int
}

type clog struct {
	grid [][]*entry
	next index
}

func NewLog() *clog {
	clog := new(clog)
	clog.grid = make([][]*entry, 0)
	clog.next = index{0, 0}
	return clog
}
