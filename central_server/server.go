package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/montanaflynn/stats"
	pb "github.gatech.edu/faasedge/feo/offloadproto"
	"google.golang.org/grpc"
)

var (
	port = flag.Int("port", 50051, "The server port")
)

const (
	//10 seconds old data is pruned from invocation history
	HISTORY_WINDOW = 10 * 1e9  // unit : ns
	FNSVCTIME      = 0.1 * 1e9 // unit : ns
)

// server is used to implement helloworld.GreeterServer.
type NodeInfo struct {
	nodename       string
	fname          string
	qlen           float32
	f_svctime      int64
	invoke_history []int64
	qlen_ctr       []int
}

func NewNodeInfo(nodename, fname string) NodeInfo {

	ni := NodeInfo{nodename: nodename, fname: fname}
	ni.f_svctime = FNSVCTIME
	ni.invoke_history = []int64{}
	ni.qlen_ctr = []int{}
	return ni
}

// func (n *NodeInfo) update_qlen(new_qlen float32) {
// 	n.qlen = n.qlen*n.qlen_alpha + (1-n.qlen_alpha)*float32(new_qlen)
// }

type server struct {
	pb.UnimplementedOffloadStateHubServer
	nodemap map[string]NodeInfo
	mapMu   sync.Mutex
}

/*
- The server maintains states and updates it every EPOCH (i.e., when the node sends over invocations over the previous epoch.)
- The approach to measuring current_capacity could be based on some sort of hysterisis from the previous_capacity
- period in seconds
*/
// func new_qlen(invoke_history []int64, period float32) float32 {

// 	qlen := float32(0)

// 	if len(invoke_history) != 0 {
// 		s_at := invoke_history[0]
// 		e_at := invoke_history[len(invoke_history)-1]
// 		history_range := e_at - s_at
// 		rps := float32(len(invoke_history)) / float32(history_range/1e9)

// 		// qlen is based on average arrival every period (e.g., 100ms (sourced from historical data per function))
// 		qlen = period * rps
// 		log.Println("[DEBUG] invoke_history", invoke_history, " qlen", qlen)

// 	}
// 	return qlen
// }

func (ni *NodeInfo) update_history(new_invoke_history []int64) {

	// prune older than HISTORY_WINDOW data points
	if len(new_invoke_history) == 0 {
		return
	}

	e_at := new_invoke_history[len(new_invoke_history)-1]
	start := 0
	for i, at := range ni.invoke_history {
		if e_at-at <= HISTORY_WINDOW {
			start = i
			break
		}
	}

	ni.invoke_history = ni.invoke_history[start:]
	ni.qlen_ctr = ni.qlen_ctr[start:]

	// update qlen_ctr with new data
	cur := len(ni.invoke_history)
	ni.invoke_history = append(ni.invoke_history, new_invoke_history...)

	for start = cur; start > 0; start-- {
		if ni.invoke_history[cur]-ni.invoke_history[start] > ni.f_svctime {
			start++
			break
		}
	}

	ctr := (cur - start) + 1
	for ; cur < len(ni.invoke_history); cur++ {

		for ; start <= cur; start++ {
			if ni.invoke_history[cur]-ni.invoke_history[start] <= ni.f_svctime {
				break
			} else {
				ctr--
			}
		}

		ni.qlen_ctr = append(ni.qlen_ctr, ctr)
		ctr++
	}

	log.Println("[DEBUG] qlen_arr, ", ni.qlen_ctr)
	first := ni.invoke_history[0]
	last := ni.invoke_history[len(ni.invoke_history)-1]
	dur := float64(last-first) / 1e10
	data := stats.LoadRawData(ni.qlen_ctr)
	p99, err := stats.Percentile(data, 99)
	if err != nil {
		log.Println("[WARNING] unable to calculate P99,", err)
	}
	ni.qlen = float32(p99)
	log.Printf("[DEBUG] dur=%f,qlen=%f\n", dur, ni.qlen)
}

func (s *server) UpdateState(ctx context.Context, in *pb.NodeState) (*pb.UpdateStateResponse, error) {

	s.mapMu.Lock()
	defer s.mapMu.Unlock()
	nodename := in.Name
	//TODO: enable multi-function support
	finfo := in.FinfoList[0]

	val, ok := s.nodemap[nodename]
	if ok {
		//val.update_history(finfo.InvokeHistory)
		val.qlen = finfo.Qlen
		s.nodemap[nodename] = val
	} else {
		s.nodemap[nodename] = NewNodeInfo(nodename, finfo.FunctionName)
	}

	return &pb.UpdateStateResponse{Success: true}, nil
}

func (s *server) GetState(ctx context.Context, in *pb.StateQuery) (*pb.StateResponse, error) {
	resp := &pb.StateResponse{}
	s.mapMu.Lock()
	resp.Nodes = []*pb.NodeState{}

	for node, info := range s.nodemap {
		ni := &pb.NodeState{}
		ni.FinfoList = []*pb.FunctionInfo{{FunctionName: info.fname, Qlen: info.qlen}}
		ni.Name = node

		resp.Nodes = append(resp.Nodes, ni)
	}
	s.mapMu.Unlock()
	return resp, nil
}

func (s *server) GetCandidate(ctx context.Context, in *pb.CandidateQuery) (*pb.CandidateResponse, error) {

	s.mapMu.Lock()
	minv := float32(10000)
	key := in.NodeName

	for k, v := range s.nodemap {
		// log.Println("[DEBUG] mapele: ", k, v.qlen)
		if v.qlen < minv {
			key = k
			minv = v.qlen
		}
	}
	s.mapMu.Unlock()
	// log.Printf("[DEBUG] Get Candidate for %s: %s\n", in.NodeName, key)

	resp := &pb.CandidateResponse{Node: key}

	return resp, nil
}

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterOffloadStateHubServer(s, &server{nodemap: make(map[string]NodeInfo)})
	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
