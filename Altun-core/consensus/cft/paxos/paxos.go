package paxos

import (
	"fmt"
	"hash/fnv"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/wtwinlab/altun/consensus/cft"
	"github.com/wtwinlab/altun/consensus/cft/auditor"
)

// forensicsCmdHash returns a stable 16-char hex digest of a command for the
// forensics log. We use the rendered %v of the Command so that any two
// equal Commands produce the same hash, which is what the verifier's
// equality-based grouping needs. Short enough to keep the forensics log
// small in high-load runs, long enough to avoid realistic collisions.
func forensicsCmdHash(cmd cft.Command) string {
	h := fnv.New64a()
	fmt.Fprintf(h, "%v", cmd)
	return fmt.Sprintf("%016x", h.Sum64())
}

// forensicsP1bLogHash hashes the "highest accepted per slot" map that an
// acceptor returns in its P1b promise. We sort by slot first so the
// hash is deterministic regardless of Go's map iteration order.
func forensicsP1bLogHash(l map[int]CommandBallot) string {
	keys := make([]int, 0, len(l))
	for k := range l {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	h := fnv.New64a()
	for _, k := range keys {
		cb := l[k]
		fmt.Fprintf(h, "%d:%d:%v;", k, cb.Ballot, cb.Command)
	}
	return fmt.Sprintf("%016x", h.Sum64())
}

// entry in log
type entry struct {
	ballot    cft.Ballot
	command   cft.Command
	commit    bool
	request   *cft.Request
	quorum    *cft.Quorum
	timestamp time.Time
}

// Paxos instance
type Paxos struct {
	cft.Node

	config []cft.ID

	log     map[int]*entry // log ordered by slot
	execute int            // next execute slot number
	active  bool           // active leader
	ballot  cft.Ballot    // highest ballot number
	slot    int            // highest slot number

	quorum   *cft.Quorum    // phase 1 quorum
	requests []*cft.Request // phase 1 pending requests

	Q1              func(*cft.Quorum) bool
	Q2              func(*cft.Quorum) bool
	ReplyWhenCommit bool
}

// NewPaxos creates new paxos instance
func NewPaxos(n cft.Node, options ...func(*Paxos)) *Paxos {
	p := &Paxos{
		Node:            n,
		log:             make(map[int]*entry, cft.GetConfig().BufferSize),
		slot:            -1,
		quorum:          cft.NewQuorum(),
		requests:        make([]*cft.Request, 0),
		Q1:              func(q *cft.Quorum) bool { return q.Majority() },
		Q2:              func(q *cft.Quorum) bool { return q.Majority() },
		ReplyWhenCommit: false,
	}

	for _, opt := range options {
		opt(p)
	}

	return p
}

// IsLeader indecates if this node is current leader
func (p *Paxos) IsLeader() bool {
	return p.active || p.ballot.ID() == p.ID()
}

// Leader returns leader id of the current ballot
func (p *Paxos) Leader() cft.ID {
	return p.ballot.ID()
}

// Ballot returns current ballot
func (p *Paxos) Ballot() cft.Ballot {
	return p.ballot
}

// SetActive sets current paxos instance as active leader
func (p *Paxos) SetActive(active bool) {
	p.active = active
}

// SetBallot sets a new ballot number
func (p *Paxos) SetBallot(b cft.Ballot) {
	p.ballot = b
}

// HandleRequest handles request and start phase 1 or phase 2
func (p *Paxos) HandleRequest(r cft.Request) {
	// log.Debugf("Replica %s received %v\n", p.ID(), r)
	if !p.active {
		p.requests = append(p.requests, &r)
		// current phase 1 pending
		if p.ballot.ID() != p.ID() {
			p.P1a()
		}
	} else {
		p.P2a(&r)
	}
}

// P1a starts phase 1 prepare
func (p *Paxos) P1a() {
	if p.active {
		return
	}
	p.ballot.Next(p.ID())
	p.quorum.Reset()
	p.quorum.ACK(p.ID())
	p.Broadcast(P1a{Ballot: p.ballot})
}

// P2a starts phase 2 accept
func (p *Paxos) P2a(r *cft.Request) {
	p.slot++
	p.log[p.slot] = &entry{
		ballot:    p.ballot,
		command:   r.Command,
		request:   r,
		quorum:    cft.NewQuorum(),
		timestamp: time.Now(),
	}
	p.log[p.slot].quorum.ACK(p.ID())
	m := P2a{
		Ballot:  p.ballot,
		Slot:    p.slot,
		Command: r.Command,
	}
	if cft.GetConfig().Thrifty {
		p.MulticastQuorum(cft.GetConfig().N()/2+1, m)
	} else {
		p.Broadcast(m)
	}

	// Equivocation injection for Exp 3A (INJECT_EQUIVOCATION=1 on the leader node).
	// Sends a subtly different command to ONE peer to create a conflicting P2a
	// for the same (ballot, slot). That one peer will LogP2a the normal
	// broadcasted cmd_hash AND (a moment later) the injected cmd_hash, so the
	// forensics log contains two distinct cmd_hash under the same (ballot, slot)
	// key and the verifier's p2a_uniqueness branch fires.
	//
	// The original implementation referenced p.config, a field that the Paxos
	// struct declares but NEVER populates anywhere in the codebase. With p.config
	// permanently nil, len(p.config) > 1 was always false and the whole block
	// was dead code. This produced forensics logs with zero conflict rows during
	// equivocation trials, so forensic-verify reported FN for every run.
	//
	// Fix: enumerate peer IDs from cft.GetConfig().Addrs (the singleton address
	// map populated at startup) instead of p.config. Sort by ID so the target
	// peer is deterministic across runs.
	if os.Getenv("INJECT_EQUIVOCATION") == "1" && p.IsLeader() {
		peers := make([]cft.ID, 0, len(cft.GetConfig().Addrs))
		for id := range cft.GetConfig().Addrs {
			if id != p.ID() {
				peers = append(peers, id)
			}
		}
		if len(peers) > 0 {
			sort.Slice(peers, func(i, j int) bool { return peers[i] < peers[j] })
			conflictCmd := r.Command
			conflictCmd.Value = append([]byte(nil), conflictCmd.Value...)
			conflictCmd.Value = append(conflictCmd.Value, byte('X'))
			conflictMsg := P2a{Ballot: p.ballot, Slot: p.slot, Command: conflictCmd}
			p.Send(peers[len(peers)-1], conflictMsg)
		}
	}
}

// HandleP1a handles P1a message
func (p *Paxos) HandleP1a(m P1a) {
	// log.Debugf("Replica %s ===[%v]===>>> Replica %s\n", m.Ballot.ID(), m, p.ID())

	// new leader
	if m.Ballot > p.ballot {
		p.ballot = m.Ballot
		p.active = false
		// TD use BackOff time or forward
		// forward pending requests to new leader
		p.forward()
		// if len(p.requests) > 0 {
		// 	defer p.P1a()
		// }
	}

	l := make(map[int]CommandBallot)
	for s := p.execute; s <= p.slot; s++ {
		if p.log[s] == nil || p.log[s].commit {
			continue
		}
		l[s] = CommandBallot{p.log[s].command, p.log[s].ballot}
	}

	p.Send(m.Ballot.ID(), P1b{
		Ballot: p.ballot,
		ID:     p.ID(),
		Log:    l,
	})

	// Forensics (Exp 3A p1b_consistency branch): record this acceptor's
	// promise. A byzantine acceptor that sends different P1b content to
	// different proposers in the same ballot would produce multiple rows
	// under the same (replica, ballot) key with different log_hash.
	auditor.LogP1b(string(p.ID()), int(p.ballot), forensicsP1bLogHash(l))
}

func (p *Paxos) update(scb map[int]CommandBallot) {
	for s, cb := range scb {
		p.slot = cft.Max(p.slot, s)
		if e, exists := p.log[s]; exists {
			if !e.commit && cb.Ballot > e.ballot {
				e.ballot = cb.Ballot
				e.command = cb.Command
			}
		} else {
			p.log[s] = &entry{
				ballot:  cb.Ballot,
				command: cb.Command,
				commit:  false,
			}
		}
	}
}

// HandleP1b handles P1b message
func (p *Paxos) HandleP1b(m P1b) {
	p.update(m.Log)

	// old message
	if m.Ballot < p.ballot || p.active {
		// log.Debugf("Replica %s ignores old message [%v]\n", p.ID(), m)
		return
	}

	// reject message
	if m.Ballot > p.ballot {
		p.ballot = m.Ballot
		p.active = false // not necessary
		// forward pending requests to new leader
		p.forward()
		// p.P1a()
	}

	// ack message
	if m.Ballot.ID() == p.ID() && m.Ballot == p.ballot {
		p.quorum.ACK(m.ID)
		if p.Q1(p.quorum) {
			p.active = true
			// propose any uncommitted entries
			for i := p.execute; i <= p.slot; i++ {
				// TD nil gap?
				if p.log[i] == nil || p.log[i].commit {
					continue
				}
				p.log[i].ballot = p.ballot
				p.log[i].quorum = cft.NewQuorum()
				p.log[i].quorum.ACK(p.ID())
				p.Broadcast(P2a{
					Ballot:  p.ballot,
					Slot:    i,
					Command: p.log[i].command,
				})
			}
			// propose new commands
			for _, req := range p.requests {
				p.P2a(req)
			}
			p.requests = make([]*cft.Request, 0)
		}
	}
}

// HandleP2a handles P2a message
func (p *Paxos) HandleP2a(m P2a) {
	// log.Debugf("Replica %s ===[%v]===>>> Replica %s\n", m.Ballot.ID(), m, p.ID())

	if m.Ballot >= p.ballot {
		p.ballot = m.Ballot
		p.active = false
		// update slot number
		p.slot = cft.Max(p.slot, m.Slot)
		// update entry
		if e, exists := p.log[m.Slot]; exists {
			if !e.commit && m.Ballot > e.ballot {
				// different command and request is not nil
				if !e.command.Equal(m.Command) && e.request != nil {
					p.Forward(m.Ballot.ID(), *e.request)
					// p.Retry(*e.request)
					e.request = nil
				}
				e.command = m.Command
				e.ballot = m.Ballot
			}
		} else {
			p.log[m.Slot] = &entry{
				ballot:  m.Ballot,
				command: m.Command,
				commit:  false,
			}
		}
	}

	p.Send(m.Ballot.ID(), P2b{
		Ballot: p.ballot,
		Slot:   m.Slot,
		ID:     p.ID(),
	})

	// Forensics (Exp 3A, Direction A):
	//   LogP2a → p2a_uniqueness branch catches leader equivocation when
	//            different acceptors record different cmd_hash for the
	//            same (ballot, slot). The INJECT_EQUIVOCATION block in
	//            P2a() is the trace generator that produces this.
	//   LogP2b → p2b_uniqueness branch catches acceptor double-voting.
	//            Acceptors should vote at most once per (ballot, slot);
	//            two distinct cmd_hash under (replica, ballot, slot) is
	//            a safety break. The `vote_flip_p2b` synthetic trace
	//            appends rows that hit this.
	cmdHash := forensicsCmdHash(m.Command)
	auditor.LogP2a(string(p.ID()), int(m.Ballot), m.Slot, cmdHash)
	auditor.LogP2b(string(p.ID()), int(m.Ballot), m.Slot, cmdHash)
}

// HandleP2b handles P2b message
func (p *Paxos) HandleP2b(m P2b) {
	// old message
	entry, exist := p.log[m.Slot]
	if !exist || m.Ballot < entry.ballot || entry.commit {
		return
	}

	// log.Debugf("Replica %s ===[%v]===>>> Replica %s\n", m.ID, m, p.ID())

	// reject message
	// node update its ballot number and falls back to acceptor
	if m.Ballot > p.ballot {
		p.ballot = m.Ballot
		p.active = false
	}

	// ack message
	// the current slot might still be committed with q2
	// if no q2 can be formed, this slot will be retried when received p2a or p3
	if m.Ballot.ID() == p.ID() && m.Ballot == p.log[m.Slot].ballot {
		p.log[m.Slot].quorum.ACK(m.ID)
		if p.Q2(p.log[m.Slot].quorum) {
			p.log[m.Slot].commit = true
			// Forensics (Exp 3A commit_divergence branch): leader-side
			// commit event. Every replica that commits this slot must
			// later log the same cmd_hash; the verifier cross-checks.
			auditor.LogCommit(string(p.ID()), m.Slot, forensicsCmdHash(p.log[m.Slot].command))
			p.Broadcast(P3{
				Ballot:  m.Ballot,
				Slot:    m.Slot,
				Command: p.log[m.Slot].command,
			})

			if p.ReplyWhenCommit {
				r := p.log[m.Slot].request
				r.Reply(cft.Reply{
					Command:   r.Command,
					Timestamp: r.Timestamp,
				})
			} else {
				p.exec()
			}
		}
	}
}

// HandleP3 handles phase 3 commit message
func (p *Paxos) HandleP3(m P3) {
	// log.Debugf("Replica %s ===[%v]===>>> Replica %s\n", m.Ballot.ID(), m, p.ID())

	p.slot = cft.Max(p.slot, m.Slot)

	e, exist := p.log[m.Slot]
	if exist {
		if !e.command.Equal(m.Command) && e.request != nil {
			// p.Retry(*e.request)
			p.Forward(m.Ballot.ID(), *e.request)
			e.request = nil
		}
	} else {
		p.log[m.Slot] = &entry{}
		e = p.log[m.Slot]
	}

	e.command = m.Command
	e.commit = true

	// Forensics (Exp 3A commit_divergence branch): follower-side commit
	// event. Same grouping as the leader's LogCommit in HandleP2b.
	auditor.LogCommit(string(p.ID()), m.Slot, forensicsCmdHash(m.Command))

	if p.ReplyWhenCommit {
		if e.request != nil {
			e.request.Reply(cft.Reply{
				Command:   e.request.Command,
				Timestamp: e.request.Timestamp,
			})
		}
	} else {
		p.exec()
	}
}

func (p *Paxos) exec() {
	for {
		e, ok := p.log[p.execute]
		if !ok || !e.commit {
			break
		}
		// log.Debugf("Replica %s execute [s=%d, cmd=%v]", p.ID(), p.execute, e.command)
		value := p.Execute(e.command)
		if e.request != nil {
			reply := cft.Reply{
				Command:    e.command,
				Value:      value,
				Properties: make(map[string]string),
			}
			reply.Properties[HTTPHeaderSlot] = strconv.Itoa(p.execute)
			reply.Properties[HTTPHeaderBallot] = e.ballot.String()
			reply.Properties[HTTPHeaderExecute] = strconv.Itoa(p.execute)
			e.request.Reply(reply)
			e.request = nil
		}
		// TD clean up the log periodically
		delete(p.log, p.execute)
		p.execute++
	}
}

// LastCommittedSlot returns the highest slot that has been executed.
func (p *Paxos) LastCommittedSlot() int {
	return p.execute - 1
}

// AllSlotsCommitted returns true if every allocated slot up to p.slot is committed.
func (p *Paxos) AllSlotsCommitted() bool {
	for i := p.execute; i <= p.slot; i++ {
		e, ok := p.log[i]
		if !ok || !e.commit {
			return false
		}
	}
	return true
}

func (p *Paxos) forward() {
	for _, m := range p.requests {
		p.Forward(p.ballot.ID(), *m)
	}
	p.requests = make([]*cft.Request, 0)
}
