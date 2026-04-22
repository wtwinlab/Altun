package epaxos

import (
	"flag"

	"github.com/wtwinlab/altun/lib"

	"github.com/wtwinlab/altun/consensus/cft"
	"github.com/wtwinlab/altun/lib/log"
)

var replyWhenCommit = flag.Bool("ReplyWhenCommit", false, "Reply to client when request is committed, instead of executed")

type Replica struct {
	cft.Node
	log          map[cft.ID]map[int]*instance
	slot         map[cft.ID]int // current instance number, start with 1
	committed    map[cft.ID]int
	executed     map[cft.ID]int
	conflicts    map[cft.ID]map[cft.Key]int
	maxSeqPerKey map[cft.Key]int

	graph *lib.Graph

	fast int
	slow int
}

// NewReplica initialize replica and register all message types
func NewReplica(id cft.ID) *Replica {
	r := &Replica{
		Node:         cft.NewNode(id),
		log:          make(map[cft.ID]map[int]*instance),
		slot:         make(map[cft.ID]int),
		committed:    make(map[cft.ID]int),
		executed:     make(map[cft.ID]int),
		conflicts:    make(map[cft.ID]map[cft.Key]int),
		maxSeqPerKey: make(map[cft.Key]int),
		graph:        lib.NewGraph(),
	}
	for id := range cft.GetConfig().Addrs {
		r.log[id] = make(map[int]*instance, cft.GetConfig().BufferSize)
		r.slot[id] = -1
		r.committed[id] = -1
		r.executed[id] = -1
		r.conflicts[id] = make(map[cft.Key]int, cft.GetConfig().BufferSize)
	}

	r.Register(cft.Request{}, r.handleRequest)
	r.Register(PreAccept{}, r.handlePreAccept)
	r.Register(PreAcceptReply{}, r.handlePreAcceptReply)
	r.Register(Accept{}, r.handleAccept)
	r.Register(AcceptReply{}, r.handleAcceptReply)
	r.Register(Commit{}, r.handleCommit)

	return r
}

// attibutes generates the sequence and dependency attributes for command
func (r Replica) attributes(cmd cft.Command) (seq int, dep map[cft.ID]int) {
	seq = 0
	dep = make(map[cft.ID]int)
	for id := range r.conflicts {
		if d, exists := r.conflicts[id][cmd.Key]; exists {
			if d > dep[id] {
				dep[id] = d
				if seq <= r.log[id][d].seq {
					seq = r.log[id][d].seq + 1
				}
			}
		}
	}
	if s, exists := r.maxSeqPerKey[cmd.Key]; exists {
		if seq <= s {
			seq = s + 1
		}
	}
	return seq, dep
}

// updates local record for conflicts
func (r *Replica) update(cmd cft.Command, id cft.ID, slot, seq int) {
	k := cmd.Key
	d, exists := r.conflicts[id][k]
	if exists {
		if d < slot {
			r.conflicts[id][k] = slot
		}
	} else {
		r.conflicts[id][k] = slot
	}
	s, exists := r.maxSeqPerKey[k]
	if exists {
		if s < seq {
			r.maxSeqPerKey[k] = seq
		}
	} else {
		r.maxSeqPerKey[k] = seq
	}
}

func (r *Replica) updateCommit(id cft.ID) {
	s := r.committed[id]
	for r.log[id][s+1] != nil && (r.log[id][s+1].status == COMMITTED || r.log[id][s+1].status == EXECUTED) {
		r.committed[id] = r.committed[id] + 1
		s = r.committed[id]
	}
	r.execute()
}

func (r *Replica) handleRequest(m cft.Request) {
	id := r.ID()
	ballot := cft.NewBallot(0, id)
	r.slot[id]++
	s := r.slot[id]
	seq, dep := r.attributes(m.Command)

	r.log[id][s] = &instance{
		cmd:     m.Command,
		ballot:  ballot,
		status:  PREACCEPTED,
		seq:     seq,
		dep:     dep,
		changed: false,
		request: &m,
		quorum:  cft.NewQuorum(),
	}

	// self ack
	r.log[id][s].quorum.ACK(id)

	r.update(m.Command, id, s, seq)

	r.Broadcast(PreAccept{
		Ballot:  ballot,
		Replica: id,
		Slot:    s,
		Command: m.Command,
		Seq:     seq,
		Dep:     r.log[id][s].copyDep(),
	})
}

func (r *Replica) handlePreAccept(m PreAccept) {
	log.Debugf("Replica %s receives PreAccept %+v", r.ID(), m)
	id := m.Replica
	s := m.Slot
	i := r.log[id][s]

	if i == nil {
		r.log[id][s] = &instance{}
		i = r.log[id][s]
	}

	if i.status == COMMITTED || i.status == ACCEPTED {
		if i.cmd.Empty() {
			i.cmd = m.Command
			r.update(m.Command, id, s, m.Seq)
		}
		return
	}

	if s > r.slot[id] {
		r.slot[id] = s
	}

	seq, dep := r.attributes(m.Command)

	if m.Ballot >= i.ballot {
		i.ballot = m.Ballot
		i.cmd = m.Command
		i.status = PREACCEPTED
		i.seq = seq
		i.dep = dep
	}

	r.update(m.Command, id, s, seq)

	c := make(map[cft.ID]int)
	for id, d := range r.committed {
		c[id] = d
	}
	r.Send(m.Replica, PreAcceptReply{
		Replica:   r.ID(),
		Slot:      s,
		Ballot:    i.ballot,
		Seq:       seq,
		Dep:       i.copyDep(),
		Committed: c,
	})
}

func (r *Replica) handlePreAcceptReply(m PreAcceptReply) {
	log.Debugf("Replica %s receives PreAcceptReply %+v", r.ID(), m)
	i := r.log[r.ID()][m.Slot]
	if i.status != PREACCEPTED {
		return
	}

	if m.Ballot > i.ballot {
		// TD merge or not
		return
	}

	i.quorum.ACK(m.Replica)
	i.merge(m.Seq, m.Dep)

	committed := true
	for id, d := range m.Committed {
		if d > r.committed[id] {
			r.committed[id] = d
		}
		if r.committed[id] >= 0 && r.committed[id] < i.dep[id] {
			committed = false
		}
	}

	if i.quorum.FastQuorum() {
		// fast path or slow path
		if !i.changed && committed {
			// fast path
			r.fast++
			log.Debugf("Replica %s number of fast instance: %d", r.ID(), r.fast)
			i.status = COMMITTED
			r.updateCommit(r.ID())
			r.Broadcast(Commit{
				Ballot:  i.ballot,
				Replica: r.ID(),
				Slot:    m.Slot,
				Command: i.cmd,
				Seq:     i.seq,
				Dep:     i.copyDep(),
			})
			if *replyWhenCommit {
				i.request.Reply(cft.Reply{
					Command: i.cmd,
				})
			}
		} else {
			// slow path
			r.slow++
			log.Debugf("Replica %s number of slow instance: %d", r.ID(), r.slow)
			i.status = ACCEPTED
			// reset quorum for accept message
			i.quorum.Reset()
			// self ack
			i.quorum.ACK(r.ID())
			r.Broadcast(Accept{
				Ballot:  i.ballot,
				Replica: r.ID(),
				Slot:    m.Slot,
				Seq:     i.seq,
				Dep:     i.copyDep(),
			})
		}
	}
}

func (r *Replica) handleAccept(m Accept) {
	log.Debugf("Replica %s receives Accept %+v", r.ID(), m)
	id := m.Replica
	s := m.Slot
	i := r.log[id][s]

	if i == nil {
		r.log[id][s] = &instance{}
		i = r.log[id][s]
	}

	if i.status == COMMITTED || i.status == EXECUTED {
		return
	}

	if s > r.slot[id] {
		r.slot[id] = s
	}

	if m.Ballot >= i.ballot {
		i.status = ACCEPTED
		i.ballot = m.Ballot
		i.seq = m.Seq
		i.dep = m.Dep
	}

	r.Send(id, AcceptReply{
		Ballot:  i.ballot,
		Replica: r.ID(),
		Slot:    s,
	})
}

func (r *Replica) handleAcceptReply(m AcceptReply) {
	log.Debugf("Replica %s receives AcceptReply %+v", r.ID(), m)
	i := r.log[r.ID()][m.Slot]

	if i.status != ACCEPTED {
		return
	}

	if i.ballot < m.Ballot {
		i.ballot = m.Ballot
		// TD
		return
	}

	i.quorum.ACK(m.Replica)
	if i.quorum.Majority() {
		i.status = COMMITTED
		r.updateCommit(r.ID())
		if *replyWhenCommit {
			i.request.Reply(cft.Reply{
				Command: i.cmd,
			})
		}
		r.Broadcast(Commit{
			Ballot:  i.ballot,
			Replica: r.ID(),
			Slot:    m.Slot,
			Command: i.cmd,
			Seq:     i.seq,
			Dep:     i.copyDep(),
		})
	}
}

func (r *Replica) handleCommit(m Commit) {
	log.Debugf("Replica %s receives Commit %+v", r.ID(), m)
	i := r.log[m.Replica][m.Slot]

	if m.Slot > r.slot[m.Replica] {
		r.slot[m.Replica] = m.Slot
	}

	if i == nil {
		r.log[m.Replica][m.Slot] = &instance{}
		i = r.log[m.Replica][m.Slot]
	}

	if m.Ballot >= i.ballot {
		i.ballot = m.Ballot
		i.cmd = m.Command
		i.status = COMMITTED
		i.seq = m.Seq
		i.dep = m.Dep
		r.update(m.Command, m.Replica, m.Slot, m.Seq)
	}

	if i.request != nil {
		// someone committed NOOP retry current request
		r.Retry(*i.request)
		i.request = nil
	}
	r.updateCommit(m.Replica)
}

// LastCommittedSlot returns the highest globally committed slot across all replicas.
func (r *Replica) LastCommittedSlot() int {
	maxCommit := -1
	for _, c := range r.committed {
		if c > maxCommit {
			maxCommit = c
		}
	}
	return maxCommit
}

// AllSlotsCommitted returns true if all allocated slots are committed.
func (r *Replica) AllSlotsCommitted() bool {
	for id, s := range r.slot {
		if r.committed[id] < s {
			return false
		}
	}
	return true
}

func (r *Replica) execute() {
	for id, log := range r.log {
		for s := r.executed[id] + 1; s <= r.slot[id]; s++ {
			i := log[s]
			if i == nil {
				continue
			}
			if i.status == EXECUTED {
				if s == r.executed[id]+1 {
					r.executed[id] = s
				}
				continue
			}
			if i.status != COMMITTED {
				break
			}
			v := r.Execute(i.cmd)
			if i.request != nil {
				i.request.Reply(cft.Reply{
					Command: i.cmd,
					Value:   v,
				})
			}
			if s == r.executed[id]+1 {
				r.executed[id] = s
			}
		}
	}
}

func (r *Replica) execute2() {
	for id, log := range r.log {
		for s := r.executed[id] + 1; s <= r.slot[id]; s++ {
			i := log[s]
			if i == nil {
				continue
			}
			if i.status == EXECUTED {
				if s == r.executed[id]+1 {
					r.executed[id] = s
				}
				continue
			}
			if i.status != COMMITTED {
				continue
			}
			v := r.Execute(i.cmd)
			if i.request != nil {
				i.request.Reply(cft.Reply{
					Command: i.cmd,
					Value:   v,
				})
				i.request = nil
			}
			if s == r.executed[id]+1 {
				r.executed[id] = s
			}
		}
	}
}
