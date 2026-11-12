package main

import (
	"context"
	"fmt"
	"reflect"

	"github.com/wtwinlab/Altun/layers/proxy/proto"
	pbv "github.com/wtwinlab/Altun/layers/proxy/proto"
	dbm "github.com/wtwinlab/Altun/layers/storage/db"
	ledgerm "github.com/wtwinlab/Altun/layers/storage/ledger"
)

type ID string
type Node interface {
	ID() string
	Run()
	Register(m interface{}, f interface{})
}

type node struct {
	id      string
	state   dbm.DB
	ledger  *ledgerm.LogLedger
	handles map[string]reflect.Value
	//server  *http.Server
	proto.UnimplementedProxyServer
}

// mustEmbedUnimplementedProxyServer implements proto.ProxyServer.
func (n *node) mustEmbedUnimplementedProxyServer() {
	panic("unimplemented")
}

func NewNode(id string) *node {
	ctx := &DBContext{ID: id}
	statedb, err := DefaultDBProvider(ctx)
	if err != nil {
		fmt.Errorf("Failed to create statedb, ", err)
	}
	ledgerdb, err := ledgerm.NewLedger("", true)
	if err != nil {
		fmt.Errorf("Failed to create ledgerdb, ", err)
	}
	return &node{
		id:      id,
		state:   statedb,
		ledger:  ledgerdb,
		handles: make(map[string]reflect.Value),
	}
}

func (n *node) ID() string {
	return n.id
}
func (n *node) Register(m interface{}, f interface{}) {
	t := reflect.TypeOf(m)
	fn := reflect.ValueOf(f)
	if fn.Kind() != reflect.Func || fn.Type().NumIn() != 1 || fn.Type().In(0) != t {
		panic("register handle function error")
	}
	n.handles[t.String()] = fn
}
func (sn *node) Get(ctx context.Context, req *pbv.GetRequest) (*pbv.GetResponse, error) {
	res, err := sn.state.Get([]byte(req.GetKey()))
	if err != nil {
		return nil, err
	}

	return &pbv.GetResponse{Value: string(res)}, nil
}

func (sn *node) Set(ctx context.Context, req *pbv.SetRequest) (*pbv.SetResponse, error) {
	// Use serverclient instance to set

	err := sn.state.Set([]byte(req.Key), []byte(req.Value))
	if err != nil {
		return nil, err
	}
	fmt.Println("Debug20220222-srv " + string(req.Key))
	return &pbv.SetResponse{}, nil
}

// func (n *node) Run() {
// 	log.Infof("node %v start running", n.id)
// 	if len(n.handles) > 0 {
// 		go n.handle()
// 		go n.recv()
// 	}
// 	n.http()
// }
