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

type HybridOffloader struct {
	*BaseOffloader
	quit           chan bool
	gap_ms         int
	wg             sync.WaitGroup
	conn           *grpc.ClientConn
	client         pb.OffloadStateHubClient
	ControllerAddr string

	//TODO: update after statemachine is updated
	invocation_history []int64
	qlen               float32
	qlenMu             sync.RWMutex
	iHistoryMu         sync.Mutex
}

func NewHybridOffloader(base *BaseOffloader) *HybridOffloader {
	fed := &HybridOffloader{BaseOffloader: base}
	fed.Qlen_max = 2
	fed.gap_ms = 1000
	fed.quit = make(chan bool)
	fed.ControllerAddr = fed.config.Controller

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

func (o *HybridOffloader) CheckAndEnq(req *http.Request) (*list.Element, bool) {
	//NOTE: only happens at the receive of a request
	o.iHistoryMu.Lock()
	o.invocation_history = append(o.invocation_history, int64(time.Now().UnixNano()))
	o.iHistoryMu.Unlock()

	ele, status := o.BaseOffloader.CheckAndEnq(req)

	return ele, status
}

func (o *HybridOffloader) update_qlen() {
	defer o.wg.Done()

	cur_qlen := o.Finfo.getSnapshot().Qlen

	o.qlenMu.Lock()
	o.qlen = 0.2*o.qlen + 0.8*float32(cur_qlen)
	o.qlenMu.Unlock()
}

func (o *HybridOffloader) buildAndSendReq() {
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

func (o *HybridOffloader) stateUpdateRoutine() {
	defer o.wg.Done()

	send_timer := time.NewTicker(time.Duration(o.gap_ms) * time.Millisecond)
	qlen_timer := time.NewTicker(time.Duration(100) * time.Millisecond)
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
		}
	}
}

func (o *HybridOffloader) Close() {
	o.conn.Close()
	close(o.quit)
	o.wg.Wait()
}

func (o *HybridOffloader) GetOffloadCandidate(req *http.Request) string {

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	creq := &pb.CandidateQuery{NodeName: o.Host}
	r, err := o.client.GetCandidate(ctx, creq)
	if err != nil {
		log.Println("Error requesting candidate")
		return o.Host
	}

	return r.GetNode()
}

func (o *HybridOffloader) MetricSMAnalyze(ctx *list.Element) {

	var timeElapsed time.Duration
	if (ctx.Value.(*MetricSM).state == FinalState) && (ctx.Value.(*MetricSM).candidate != "default") {
		if ctx.Value.(*MetricSM).local {
			timeElapsed = ctx.Value.(*MetricSM).postLocal.Sub(ctx.Value.(*MetricSM).preLocal)
		} else {
			timeElapsed = ctx.Value.(*MetricSM).postOffload.Sub(ctx.Value.(*MetricSM).preOffload)
		}
		ctx.Value.(*MetricSM).elapsed = timeElapsed
	}
}
