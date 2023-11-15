package main

import (
	"container/list"
	"log"
	"math"
	"net/http"
	"sync"
	"time"
)

type ImpedenceOffloader struct {
	*BaseOffloader //hacky embedding, because you cannot override methods of an embedding
	alpha	float64
	candidateToIndex map[string]int
	mu      sync.Mutex
}

func NewImpedenceOffloader(base *BaseOffloader) *ImpedenceOffloader {
	impedenceOffloader := &ImpedenceOffloader{alpha: 0.2, BaseOffloader: base}
	impedenceOffloader.candidateToIndex = make(map[string]int)

	for idx,router := range base.RouterList {
		impedenceOffloader.candidateToIndex[router.host] = idx
	}

	return impedenceOffloader
}

func (o *ImpedenceOffloader) CheckAndEnq(req *http.Request) (*list.Element, bool) {
	//NOTE: in round robin checkAndEnq returns true AND does a forceenq IF it is an offloaded request.
	var ctx *list.Element
	enq_success := false
	if o.IsOffloaded(req) {
		log.Println("[INFO] Already offloaded. Force Enq")
		ctx = o.ForceEnq(req)
		enq_success = true
	}
	return ctx, enq_success
}

func (o *ImpedenceOffloader) GetOffloadCandidate(req *http.Request) string {
	o.mu.Lock()
	defer o.mu.Unlock()

	total_nodes := len(o.RouterList)

	minWeight := math.MaxFloat64
	minIndex := -1

	for i:=0; i<total_nodes; i++ {
		if (minWeight > o.RouterList[i].weight) {
			minIndex = i
			minWeight = o.RouterList[i].weight
		}
	}

	if (minIndex == -1) {
		return o.Host
	} else {
		return o.RouterList[minIndex].host
	}
}

// func (o *ImpedenceOffloader) PreProxyMetric(req *http.Request, candidate string) interface{} {
// 	return time.Now()
// }

// func (o *ImpedenceOffloader) PostProxyMetric(req *http.Request, candidate string, preProxyMetric interface{}) {
// 	// Asserting that preProxyMetric holds time.Time value.
// 	timeElapsed := time.Since(preProxyMetric.(time.Time))
// 	candidateIdx := o.candidateToIndex[candidate]

// 	o.mu.Lock()
// 	defer o.mu.Unlock()

// 	prevRouterWeight := o.RouterList[candidateIdx].weight
// 	o.RouterList[candidateIdx].weight = prevRouterWeight * (1 - o.alpha) + float64(timeElapsed.Microseconds()/1000) * o.alpha
// }

func (o *ImpedenceOffloader) MetricSMAnalyze(ctx *list.Element) {

	var timeElapsed time.Duration
	if (ctx.Value.(*MetricSM).state == FinalState) {
		if (ctx.Value.(*MetricSM).local) {
			timeElapsed = ctx.Value.(*MetricSM).postLocal.Sub(ctx.Value.(*MetricSM).preLocal)
		} else {
			timeElapsed = ctx.Value.(*MetricSM).postOffload.Sub(ctx.Value.(*MetricSM).preOffload)
		}

		if (ctx.Value.(*MetricSM).candidate != "default") {
			candidateIdx := o.candidateToIndex[ctx.Value.(*MetricSM).candidate]

			o.mu.Lock()
			defer o.mu.Unlock()
			
			prevRouterWeight := o.RouterList[candidateIdx].weight
			o.RouterList[candidateIdx].weight = prevRouterWeight * (1 - o.alpha) + float64(timeElapsed.Microseconds()/1000) * o.alpha
		}
	}
}