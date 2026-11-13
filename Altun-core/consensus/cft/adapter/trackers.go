package adapter

import (
	"github.com/wtwinlab/altun/consensus/cft/epaxos"
	"github.com/wtwinlab/altun/consensus/cft/vpaxos"
	"github.com/wtwinlab/altun/consensus/cft/wpaxos"
)

// EPaxosTracker adapts EPaxos commit tracking to the CommitTracker interface.
type EPaxosTracker struct {
	R *epaxos.Replica
}

func (t *EPaxosTracker) LastCommittedSlot() int {
	return t.R.LastCommittedSlot()
}

func (t *EPaxosTracker) AllSlotsCommitted() bool {
	return t.R.AllSlotsCommitted()
}

// VPaxosTracker adapts VPaxos commit tracking to the CommitTracker interface.
type VPaxosTracker struct {
	R *vpaxos.Replica
}

func (t *VPaxosTracker) LastCommittedSlot() int {
	return t.R.LastCommittedSlot()
}

func (t *VPaxosTracker) AllSlotsCommitted() bool {
	return t.R.AllSlotsCommitted()
}

// WPaxosTracker adapts WPaxos commit tracking to the CommitTracker interface.
type WPaxosTracker struct {
	R *wpaxos.Replica
}

func (t *WPaxosTracker) LastCommittedSlot() int {
	return t.R.LastCommittedSlot()
}

func (t *WPaxosTracker) AllSlotsCommitted() bool {
	return t.R.AllSlotsCommitted()
}
