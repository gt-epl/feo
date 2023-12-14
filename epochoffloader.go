package main

import (
	"container/list"
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	pb "github.gatech.edu/faasedge/feo/offloadproto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type EpochOffloader struct {
	*BaseOffloader
	quit           chan bool
	gap_ms         int
	epoch_ms       int
	wg             sync.WaitGroup
	conn           *grpc.ClientConn
	client         pb.OffloadStateHubClient
	ControllerAddr string

	//TODO: update after statemachine is updated
	invocation_history []int64
	qlen               float32
	qlenMu             sync.RWMutex
	iHistoryMu         sync.Mutex
	nodemap            map[string]float32
}

func NewEpochOffloader(base *BaseOffloader) *EpochOffloader {
	fed := &EpochOffloader{BaseOffloader: base}
	fed.Qlen_max = 2
	fed.gap_ms = 1000
	fed.epoch_ms = 2000
	fed.quit = make(chan bool)
	fed.ControllerAddr = fed.config.Controller
	fed.nodemap = make(map[string]float32)

	//setup connection with controller
	var err error
	fed.conn, err = grpc.Dial(fed.ControllerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	fed.client = pb.NewOffloadStateHubClient(fed.conn)

	fed.wg.Add(1)
	go fed.stateUpdateRoutine()
	return fed
}

func (o *EpochOffloader) CheckAndEnq(req *http.Request) (*list.Element, bool) {
	//NOTE: only happens at the receive of a request
	o.iHistoryMu.Lock()
	o.invocation_history = append(o.invocation_history, int64(time.Now().UnixNano()))
	o.iHistoryMu.Unlock()

	ele, status := o.BaseOffloader.CheckAndEnq(req)

	return ele, status
}

func (o *EpochOffloader) update_qlen() {
	defer o.wg.Done()

	cur_qlen := o.Finfo.getSnapshot().Qlen

	o.qlenMu.Lock()
	o.qlen = 0.2*o.qlen + 0.8*float32(cur_qlen)
	o.qlenMu.Unlock()
}

func (o *EpochOffloader) sync_state() {
	defer o.wg.Done()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	req := &pb.StateQuery{NodeName: o.Host}
	r, err := o.client.GetState(ctx, req)
	if err != nil {
		log.Println("[WARNING] coud not sync state", err)
	}

	for _, ni := range r.Nodes {
		o.nodemap[ni.Name] = ni.FinfoList[0].Qlen
	}
}

func (o *EpochOffloader) buildAndSendReq() {
	defer o.wg.Done()
	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req := &pb.NodeState{Name: o.Host}

	fi := &pb.FunctionInfo{FunctionName: o.Finfo.name}

	o.iHistoryMu.Lock()
	fi.InvokeHistory = make([]int64, len(o.invocation_history))
	copy(fi.InvokeHistory, o.invocation_history)
	//prune invocation history
	o.invocation_history = []int64{}
	o.iHistoryMu.Unlock()

	o.qlenMu.RLock()
	fi.Qlen = o.qlen
	o.qlenMu.RUnlock()

	//TODO: multi-function supported does not exist as of yet
	req.FinfoList = append(req.FinfoList, fi)

	r, err := o.client.UpdateState(ctx, req)
	if err != nil {
		log.Printf("[WARNING] could not send state: %v", err)
	}
	if !r.GetSuccess() {
		log.Println("[WARNING] controller responded negatively")
	}
}

func (o *EpochOffloader) stateUpdateRoutine() {
	defer o.wg.Done()

	send_timer := time.NewTicker(time.Duration(o.gap_ms) * time.Millisecond)
	qlen_timer := time.NewTicker(time.Duration(100) * time.Millisecond)
	epoch_timer := time.NewTicker(time.Duration(o.epoch_ms) * time.Millisecond)
	for {
		select {
		case <-o.quit:
			return
		case <-send_timer.C:
			o.wg.Add(1)
			go o.buildAndSendReq()
		case <-qlen_timer.C:
			o.wg.Add(1)
			go o.update_qlen()
		case <-epoch_timer.C:
			o.wg.Add(1)
			go o.sync_state()
		}
	}
}

func (o *EpochOffloader) Close() {
	o.conn.Close()
	close(o.quit)
	o.wg.Wait()
}

func (o *EpochOffloader) GetOffloadCandidate(req *http.Request) string {
	minv := float32(10000)
	var candidate string

	for k, v := range o.nodemap {
		// log.Println("[DEBUG] mapele: ", k, v.qlen)
		if v < minv {
			candidate = k
			minv = v
		}
	}
	return candidate
}
