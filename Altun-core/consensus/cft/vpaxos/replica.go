package vpaxos

import (
	"flag"

	"github.com/wtwinlab/altun/consensus/cft"
	"github.com/wtwinlab/altun/lib/log"
)

// master node id
var mid = cft.ID(*flag.String("mid", "2.1", "Master ID in format of Zone.Node"))

type Replica struct {
	cft.Node

	paxos  *paxos
	index  map[cft.Key]cft.Ballot
	policy map[cft.Key]cft.Policy

	pending map[cft.Key][]cft.Request
	master  *master
}

func NewReplica(id cft.ID) *Replica {
	r := &Replica{
		Node:    cft.NewNode(id),
		index:   make(map[cft.Key]cft.Ballot),
		policy:  make(map[cft.Key]cft.Policy),
		pending: make(map[cft.Key][]cft.Request),
	}
	r.paxos = newPaxos(r.Node)

	if r.ID() == mid {
		r.master = newMaster(r)
	}

	r.Register(cft.Request{}, r.handleRequest)
	r.Register(Info{}, r.handleInfo)
	r.Register(P1a{}, r.handleP1a)
	return r
}

func (r *Replica) monitor(k cft.Key, id cft.ID) {
	_, exist := r.policy[k]
	if !exist {
		r.policy[k] = cft.NewPolicy()
	}
	to := r.policy[k].Hit(id)
	if to != "" && to.Zone() != r.ID().Zone() {
		move := Move{
			Key:  k,
			From: r.ID(),
			To:   to,
		}
		if r.master == nil {
			r.Send(mid, move)
		} else {
			r.master.handleMove(move)
		}
	}
}

func (r *Replica) handleRequest(m cft.Request) {
	log.Debugf("replica %v received %v", r.ID(), m)
	k := m.Command.Key
	b, exist := r.index[k]
	if !exist {
		if r.master == nil {
			// if unknown key, save request and query to master
			_, ok := r.pending[k]
			if !ok {
				r.pending[k] = make([]cft.Request, 0)
			}
			r.pending[k] = append(r.pending[k], m)
			r.Send(mid, Query{
				Key: k,
				ID:  r.ID(),
			})
			return
		}
		b = r.master.query(k, r.ID())
		r.index[k] = b
		if b.ID() == r.ID() {
			r.paxos.ballot = b
			r.paxos.active = true
		}
	}
	if b.ID() == r.ID() {
		r.paxos.handleRequest(m)
		r.monitor(k, m.NodeID)
	} else {
		r.Forward(b.ID(), m)
	}
}

func (r *Replica) handleInfo(m Info) {
	log.Debugf("replica %v received %v", r.ID(), m)
	r.index[m.Key] = m.Ballot
	if m.Ballot.ID() == r.ID() {
		r.paxos.ballot = m.Ballot
		if m.OldBallot == 0 {
			r.paxos.active = true
		} else {
			r.paxos.active = false
			zone := m.OldBallot.ID().Zone()
			r.MulticastZone(zone, P1a{
				Key:    m.Key,
				Ballot: m.Ballot,
			})
		}
	}
	if len(r.pending[m.Key]) > 0 {
		for _, request := range r.pending[m.Key] {
			r.handleRequest(request)
		}
		r.pending[m.Key] = make([]cft.Request, 0)
	}
}

// LastCommittedSlot returns the highest executed slot in the internal paxos log.
func (r *Replica) LastCommittedSlot() int {
	return r.paxos.execute - 1
}

// AllSlotsCommitted returns true if all allocated slots are committed.
func (r *Replica) AllSlotsCommitted() bool {
	for i := r.paxos.execute; i <= r.paxos.slot; i++ {
		e, ok := r.paxos.log[i]
		if !ok || !e.commit {
			return false
		}
	}
	return true
}

func (r *Replica) handleP1a(m P1a) {
	r.Send(m.Ballot.ID(), P1b{
		Key:    m.Key,
		Ballot: m.Ballot,
		ID:     r.ID(),
	})
}
