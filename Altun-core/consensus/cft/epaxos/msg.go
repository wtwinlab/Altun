package epaxos

import (
	"encoding/gob"
	"fmt"

	"github.com/wtwinlab/altun/consensus/cft"
)

func init() {
	gob.Register(PreAccept{})
	gob.Register(PreAcceptReply{})
	gob.Register(Accept{})
	gob.Register(AcceptReply{})
	gob.Register(Commit{})
}

type PreAccept struct {
	Ballot  cft.Ballot
	Replica cft.ID
	Slot    int
	Command cft.Command
	Seq     int
	Dep     map[cft.ID]int
}

func (m PreAccept) String() string {
	return fmt.Sprintf("PreAccept {bal=%d id=%s s=%d cmd=%v seq=%d dep=%v}", m.Ballot, m.Replica, m.Slot, m.Command, m.Seq, m.Dep)
}

type PreAcceptReply struct {
	Ballot    cft.Ballot
	Replica   cft.ID
	Slot      int
	Seq       int
	Dep       map[cft.ID]int
	Committed map[cft.ID]int
}

func (m PreAcceptReply) String() string {
	return fmt.Sprintf("PreAcceptReply {bal=%d id=%s s=%d seq=%d dep=%v c=%v}", m.Ballot, m.Replica, m.Slot, m.Seq, m.Dep, m.Committed)
}

type Accept struct {
	Ballot  cft.Ballot
	Replica cft.ID
	Slot    int
	Seq     int
	Dep     map[cft.ID]int
}

type AcceptReply struct {
	Ballot  cft.Ballot
	Replica cft.ID
	Slot    int
}

type Commit struct {
	Ballot  cft.Ballot
	Replica cft.ID
	Slot    int
	Command cft.Command
	Seq     int
	Dep     map[cft.ID]int
}
