package wpaxos

import (
	"github.com/wtwinlab/altun/consensus/cft"
	"github.com/wtwinlab/altun/consensus/cft/paxos"
)

type kpaxos struct {
	cft.Node
	key cft.Key
	*paxos.Paxos
	cft.Policy
}

func Q1(q *cft.Quorum) bool {
	if *fz == 0 {
		return q.GridRow()
	}
	return q.FGridQ1(*fz)
}

func Q2(q *cft.Quorum) bool {
	if *fz == 0 {
		return q.GridColumn()
	}
	return q.FGridQ2(*fz)
}

func newKPaxos(key cft.Key, node cft.Node) *kpaxos {
	k := &kpaxos{}
	k.Node = node
	k.key = key
	k.Policy = cft.NewPolicy()

	quorum := func(p *paxos.Paxos) {
		p.Q1 = Q1
		p.Q2 = Q2
	}
	k.Paxos = paxos.NewPaxos(k, quorum)

	// zone := int(key)%cft.GetConfig().Z() + 1
	// id := cft.NewID(zone, 1)
	// k.Paxos.SetBallot(cft.NewBallot(1, id))
	// if node.ID() == id {
	// 	k.Paxos.SetActive(true)
	// }
	return k
}

// Broadcast overrides Socket interface in Node
func (k *kpaxos) Broadcast(m interface{}) {
	switch m := m.(type) {
	case paxos.P1a:
		k.Node.Broadcast(Prepare{k.key, m})
	case paxos.P2a:
		k.Node.Broadcast(Accept{k.key, m})
	case paxos.P3:
		k.Node.Broadcast(Commit{k.key, m})
	default:
		k.Node.Broadcast(m)
	}
}

// Send overrides Socket interface in Node
func (k *kpaxos) Send(to cft.ID, m interface{}) {
	switch m := m.(type) {
	case paxos.P1b:
		k.Node.Send(to, Promise{k.key, m})
	case paxos.P2b:
		k.Node.Send(to, Accepted{k.key, m})
	default:
		k.Node.Send(to, m)
	}
}
