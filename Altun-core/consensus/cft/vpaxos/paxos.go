package vpaxos

import (
	"github.com/wtwinlab/Altun/layers/consensus/cft"
	"github.com/wtwinlab/Altun/layers/consensus/lib/log"
)

// entry in log
type entry struct {
	ballot  cft.Ballot
	command cft.Command
	commit  bool
	request *cft.Request
	quorum  *cft.Quorum
}

type paxos struct {
	cft.Node

	log     map[int]*entry
	ballot  cft.Ballot
	slot    int
	execute int
	active  bool

	pending []cft.Request // pending during phase-1
	quorum  *cft.Quorum
}

func newPaxos(node cft.Node) *paxos {
	v := &paxos{
		Node: node,
		log:  make(map[int]*entry),
		// ballot: cft.NewBallot(1, cft.NewID(node.ID().Zone(), 1)),
		slot:    -1,
		pending: make([]cft.Request, 0),
		quorum:  cft.NewQuorum(),
	}

	v.Register(P1b{}, v.handleP1b)
	v.Register(P2a{}, v.handleP2a)
	v.Register(P2b{}, v.handleP2b)
	v.Register(P3{}, v.handleP3)

	return v
}

func (p *paxos) handleRequest(m cft.Request) {
	if !p.active {
		p.pending = append(p.pending, m)
		return
	}

	if p.ballot.ID() != p.ID() {
		p.Forward(p.ballot.ID(), m)
	}

	p.slot++
	p.log[p.slot] = &entry{
		ballot:  p.ballot,
		command: m.Command,
		request: &m,
		quorum:  cft.NewQuorum(),
	}
	p.log[p.slot].quorum.ACK(p.ID())
	p.MulticastZone(p.ID().Zone(), P2a{
		Ballot:  p.ballot,
		Slot:    p.slot,
		Command: m.Command,
	})
}

func (p *paxos) handleP1b(m P1b) {
	log.Debugf("replica %v received %v", p.ID(), m)
	if p.active || m.Ballot < p.ballot {
		return
	}

	p.quorum.ACK(m.ID)
	if p.quorum.ZoneMajority() {
		p.quorum.Reset()
		p.active = true
		for _, r := range p.pending {
			p.handleRequest(r)
		}
		p.pending = nil
	}
}

func (p *paxos) handleP2a(m P2a) {
	log.Debugf("replica %v received %v", p.ID(), m)
	if m.Ballot >= p.ballot {
		p.ballot = m.Ballot
		p.slot = cft.Max(p.slot, m.Slot)
		p.log[m.Slot] = &entry{
			ballot:  m.Ballot,
			command: m.Command,
			commit:  false,
		}
	}

	p.Send(m.Ballot.ID(), P2b{
		Ballot: p.ballot,
		Slot:   m.Slot,
		ID:     p.ID(),
	})
}

func (p *paxos) handleP2b(m P2b) {
	log.Debugf("replica %v received %v", p.ID(), m)
	if m.Ballot < p.log[m.Slot].ballot || p.log[m.Slot].commit {
		return
	}

	if m.Ballot > p.ballot {
		p.ballot = m.Ballot
	}

	if m.Ballot.ID() == p.ID() && m.Ballot == p.log[m.Slot].ballot {
		p.log[m.Slot].quorum.ACK(m.ID)
		if p.log[m.Slot].quorum.ZoneMajority() {
			p.log[m.Slot].commit = true
			p.MulticastZone(p.ID().Zone(), P3{
				Ballot:  m.Ballot,
				Slot:    m.Slot,
				Command: p.log[m.Slot].command,
			})

			p.exec()
		}
	}
}

func (p *paxos) handleP3(m P3) {
	log.Debugf("replica %v received %v", p.ID(), m)
	p.slot = cft.Max(p.slot, m.Slot)
	e, exist := p.log[m.Slot]
	if !exist {
		p.log[m.Slot] = &entry{}
		e = p.log[m.Slot]
	}

	e.command = m.Command
	e.commit = true

	p.exec()
}

func (p *paxos) exec() {
	for {
		e, ok := p.log[p.execute]
		if !ok || !e.commit {
			break
		}
		value := p.Execute(e.command)
		if e.request != nil {
			e.request.Reply(cft.Reply{
				Command: e.command,
				Value:   value,
			})
			e.request = nil
		}
		p.execute++
	}
}
