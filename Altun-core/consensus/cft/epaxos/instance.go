package epaxos

import "github.com/wtwinlab/altun/consensus/cft"

type status int8

const (
	NONE status = iota
	PREACCEPTED
	ACCEPTED
	COMMITTED
	EXECUTED
)

type instance struct {
	cmd    cft.Command
	ballot cft.Ballot
	status status
	seq    int
	dep    map[cft.ID]int

	// leader bookkeeping
	request *cft.Request
	quorum  *cft.Quorum
	changed bool // seq and dep changed
}

// merge the seq and dep for instance
func (i *instance) merge(seq int, dep map[cft.ID]int) {
	if seq > i.seq {
		i.seq = seq
		i.changed = true
	}
	for id, d := range dep {
		if d > i.dep[id] {
			i.dep[id] = d
			i.changed = true
		}
	}
}

// copyDep clones dependency list of instance
func (i *instance) copyDep() (dep map[cft.ID]int) {
	dep = make(map[cft.ID]int)
	for id, d := range i.dep {
		dep[id] = d
	}
	return dep
}
