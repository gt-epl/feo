package main

import (
	"container/list"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

type router struct {
	scheme string
	host   string
}

type OffloadPolicy string

const (
	OffloadRoundRobin 	= "roundrobin"
	OffloadRandom     	= "random"
	OffloadFederated  	= "federated"
	OffloadCentral    	= "central"
	OffloadHybrid     	= "hybrid"
	OffloadImpedence 	= "impedence"
	RandomProportional 	= "randomproportional"
	RRLatency 			= "roundrobinlatency"
	OffloadEpoch		= "epoch"
)

type MetricSMState string

const (
	InitState 			= "INIT"
	PreLocalState		= "PRELOCAL"
	OffloadSearchState	= "OFFLOADSEARCH"
	PreOffloadState		= "PREOFFLOAD"
	PostOffloadState	= "POSTOFFLOAD"
	PostLocalState		= "POSTLOCAL"
	FinalState			= "FINAL"
)

const (
	OffloadSuccess = "Offload-Success"
	NodeStatus     = "Node-Status"
)

func OffloadFactory(pol OffloadPolicy, config FeoConfig) OffloaderIntf {
	base := NewBaseOffloader(config)
	switch pol {
	case OffloadRoundRobin:
		log.Println("[INFO] Selecting RoundRobin Offloader")
		return NewRoundRobinOffloader(base)
	case OffloadRandom:
		log.Println("[INFO] Selecting Random Offloader")
		return NewRandomOffloader(base)
	case OffloadFederated:
		log.Println("[INFO] Selecting Federated Offloader")
		return NewFederatedOffloader(base)
	case OffloadHybrid:
		log.Println("[INFO] Selecting Hybrid Offloader")
		return NewHybridOffloader(base)
	case OffloadCentral:
		log.Println("[INFO] Selecting Central Offloader")
		return NewCentralizedOffloader(base)
	case OffloadImpedence:
		log.Println("[INFO] Selecting Impedence Offloader")
		return NewImpedenceOffloader(base)
	case RandomProportional:
		log.Println("[INFO] Selecting Radnom Proportional Offloader")
		return NewRandomPropOffloader(base)
	case RRLatency:
		log.Println("[INFO] Selecting Round Robin Latency Offloader")
		return NewRRLatencyOffloader(base)
	case OffloadEpoch:
		log.Println("[INFO] Selecting Epoch Offloader")
		return NewEpochOffloader(base)
	default:
		log.Println("[WARNING] No policy specified. Selecting Base Offloader")
		return base
	}
}

type MetricSM struct {
	state 			MetricSMState

	init			time.Time
	preLocal		time.Time
	offloadSearch	time.Time
	preOffload		time.Time
	postOffload		time.Time
	postLocal		time.Time
	final			time.Time

	elapsed			time.Duration

	candidate		string
	local			bool
	localByDefault	bool
	localAfterFail	bool
}

type FunctionInfo struct {
	name        string
	invoke_list *list.List
	mu          sync.Mutex
}

func (f *FunctionInfo) getSnapshot() Snapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return Snapshot{
		Name: f.name,
		Qlen: float32(f.invoke_list.Len()),
	}
}

type Snapshot struct {
	Name        string  `json:"name"`
	Qlen        float32 `json:"qlen"`
	HasCapacity bool    `json:"hascapacity"`
}

type OffloaderIntf interface {
	CheckAndEnq(req *http.Request) (*list.Element, bool)
	ForceEnq(req *http.Request) *list.Element
	Deq(req *http.Request, ctx *list.Element)
	IsOffloaded(req *http.Request) bool
	GetOffloadCandidate(req *http.Request) string
	GetStatusStr() string
	PostOffloadUpdate(snap Snapshot, target string)
	Close()
	MetricSMInit() *list.Element
	MetricSMAnalyze(ctx *list.Element)
	MetricSMAdvance(ctx *list.Element, state MetricSMState, candidate ...string)
	MetricSMElapsed(ctx *list.Element) string
	MetricSMDelete(ctx *list.Element)
}

// TODO: This is single function currently. Provide multi-function support.
type BaseOffloader struct {
	Finfo      FunctionInfo
	Host       string
	RouterList []router
	Qlen_max   int32
	config     FeoConfig
	MetricSMList	*list.List
	MetricSMMu		sync.Mutex
}

func NewBaseOffloader(config FeoConfig) *BaseOffloader {
	routerList := []router{}
	for _, ip := range config.Peers {
		routerList = append(routerList, router{host: ip})
	}
	o := BaseOffloader{Host: config.Host, RouterList: routerList, Qlen_max: math.MaxInt32, config: config}
	o.Finfo.invoke_list = list.New()
	o.MetricSMList = list.New()
	return &o
}

func (o *BaseOffloader) CheckAndEnq(req *http.Request) (*list.Element, bool) {
	log.Println("[DEBUG] in checkAndEnq")
	o.Finfo.mu.Lock()
	defer o.Finfo.mu.Unlock()
	log.Println("[DEBUG ]qlen: ", o.Finfo.invoke_list.Len())
	log.Println("[DEBUG] qlen_max", int(o.Qlen_max))
	if o.Finfo.invoke_list.Len() < int(o.Qlen_max) {
		log.Println("[DEBUG] inside if branch", int(o.Qlen_max))
		return o.Finfo.invoke_list.PushBack(time.Now()), true
	} else {
		return nil, false
	}
}

func (o *BaseOffloader) IsOffloaded(req *http.Request) bool {
	forwardedField := req.Header.Get("X-Offloaded-For")
	log.Println("[DEBUG]isOffloaded ", forwardedField)
	return len(strings.TrimSpace(forwardedField)) != 0
}

func (o *BaseOffloader) ForceEnq(req *http.Request) *list.Element {
	//template string (/api/v1/namespaces/guest/actions/copy)
	o.Finfo.mu.Lock()
	defer o.Finfo.mu.Unlock()
	o.Finfo.name = strings.Split(req.URL.Path, "/")[5]
	ctx := o.Finfo.invoke_list.PushBack(time.Now())
	return ctx
}

func (o *BaseOffloader) Deq(req *http.Request, ctx *list.Element) {
	o.Finfo.mu.Lock()
	defer o.Finfo.mu.Unlock()
	o.Finfo.invoke_list.Remove(ctx)
}

func (o *BaseOffloader) GetOffloadCandidate(req *http.Request) string {
	return ""
}

func (o *BaseOffloader) GetStatusStr() string {
	log.Println("[DEBUG] in getStatusStr")
	snap := o.Finfo.getSnapshot()
	snap.HasCapacity = false
	jbytes, err := json.Marshal(snap)
	if err != nil {
		log.Println("[WARNING] could not marshall status: ", err)
	}
	return string(jbytes)
}

func (o *BaseOffloader) Close() {

}

func (o *BaseOffloader) MetricSMInit() *list.Element {
	metricSM := &MetricSM{}
	metricSM.init = time.Now()
	metricSM.state = InitState
	metricSM.candidate = "default"
	metricSM.local = false
	metricSM.localAfterFail = false
	metricSM.localByDefault = false

	o.MetricSMMu.Lock()
	defer o.MetricSMMu.Unlock()
	ctx := o.MetricSMList.PushBack(metricSM)
	return ctx
}

func (o *BaseOffloader) MetricSMAdvance(ctx *list.Element, state MetricSMState, candidate ...string) {
	// NEED ADVANCING LOGIC, SO THAT THE OFFLOADERS CAN USE THE STATE.

	for _,name := range candidate {
		if (name != "default") {
			ctx.Value.(*MetricSM).candidate = name
		}
	}

	switch(state) {
	case InitState:
		ctx.Value.(*MetricSM).state = state
		ctx.Value.(*MetricSM).init = time.Now()
	case OffloadSearchState:
		if (ctx.Value.(*MetricSM).state == InitState) {
			ctx.Value.(*MetricSM).state = state
			ctx.Value.(*MetricSM).offloadSearch = time.Now()
		}
	case PreLocalState:
		if ((ctx.Value.(*MetricSM).state == InitState) || (ctx.Value.(*MetricSM).state == OffloadSearchState) || (ctx.Value.(*MetricSM).state == PreOffloadState)) {
			if ((ctx.Value.(*MetricSM).state == InitState)) {
				ctx.Value.(*MetricSM).localByDefault = true
			}
			if ((ctx.Value.(*MetricSM).state == PreOffloadState)) {
				ctx.Value.(*MetricSM).localAfterFail = true
			}
			ctx.Value.(*MetricSM).state = state
			ctx.Value.(*MetricSM).preLocal = time.Now()
		}
	case PostLocalState:
		if (ctx.Value.(*MetricSM).state == PreLocalState) {
			ctx.Value.(*MetricSM).state = state
			ctx.Value.(*MetricSM).local = true
			ctx.Value.(*MetricSM).postLocal = time.Now()
		}
	case PreOffloadState:
		if (ctx.Value.(*MetricSM).state == OffloadSearchState) {
			ctx.Value.(*MetricSM).state = state
			ctx.Value.(*MetricSM).preOffload = time.Now()
		}
	case PostOffloadState:
		if (ctx.Value.(*MetricSM).state == PreOffloadState) {
			ctx.Value.(*MetricSM).state = state
			ctx.Value.(*MetricSM).local = false
			ctx.Value.(*MetricSM).postOffload = time.Now()
		}
	case FinalState:
		if ((ctx.Value.(*MetricSM).state == PostOffloadState) || (ctx.Value.(*MetricSM).state == PostLocalState)) {
			ctx.Value.(*MetricSM).state = state
			ctx.Value.(*MetricSM).final = time.Now()
		}
	}

	// Better
	// switch(ctx.Value.(*MetricSM).state){
	// case InitState:
	// 	ctx.Value.(*MetricSM).init = time.Now()
	// case PreLocalState:
	// 	ctx.Value.(*MetricSM).preLocal = time.Now()
	// case OffloadSearchState:
	// 	ctx.Value.(*MetricSM).offloadSearch = time.Now()
	// case PreOffloadState:
	// 	ctx.Value.(*MetricSM).preOffload = time.Now()
	// case PostOffloadState:
	// 	ctx.Value.(*MetricSM).postOffload = time.Now()
	// case PostLocalState:
	// 	ctx.Value.(*MetricSM).postLocal = time.Now()
	// case FinalState:
	// 	ctx.Value.(*MetricSM).final = time.Now()
	// }
}

func (o *BaseOffloader) MetricSMAnalyze(ctx *list.Element) {
	return
}

func (o *BaseOffloader) MetricSMDelete(ctx *list.Element) {
	o.MetricSMMu.Lock()
	defer o.MetricSMMu.Unlock()
	o.MetricSMList.Remove(ctx)

}

func (o *BaseOffloader) MetricSMElapsed(ctx *list.Element) string {
	if ((ctx.Value.(*MetricSM).state == FinalState) && (ctx.Value.(*MetricSM).candidate != "default")) {
		return ctx.Value.(*MetricSM).elapsed.String()
	} else {
		return ""
	}
}

func (o *BaseOffloader) PostOffloadUpdate(snap Snapshot, targte string) {

}
