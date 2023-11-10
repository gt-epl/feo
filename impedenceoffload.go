package main

import (
	"container/list"
	"log"
	"math"
	"net/http"
	"time"
)

type ImpedenceOffloader struct {
	base    *BaseOffloader //hacky embedding, because you cannot override methods of an embedding
	alpha	float64
	candidateToIndex map[string]int
}

func newImpedenceOffloader(base *BaseOffloader) *ImpedenceOffloader {
	impedenceOffloader := &ImpedenceOffloader{alpha: 0.2, base: base}
	impedenceOffloader.candidateToIndex = make(map[string]int)

	for idx,router := range base.RouterList {
		impedenceOffloader.candidateToIndex[router.host] = idx
	}

	return impedenceOffloader
}

func (o *ImpedenceOffloader) checkAndEnq(req *http.Request) (*list.Element, bool) {
	//NOTE: in round robin checkAndEnq returns true AND does a forceenq IF it is an offloaded request.
	var ctx *list.Element
	enq_success := false
	if o.base.isOffloaded(req) {
		log.Println("[INFO] Already offloaded. Force Enq")
		ctx = o.base.forceEnq(req)
		enq_success = true
	}
	return ctx, enq_success
}

func (o *ImpedenceOffloader) getOffloadCandidate(req *http.Request) string {
	total_nodes := len(o.base.RouterList)

	minWeight := math.MaxFloat64
	minIndex := -1

	for i:=0; i<total_nodes; i++ {
		if (minWeight > o.base.RouterList[i].weight) {
			minIndex = i
			minWeight = o.base.RouterList[i].weight
		}
	}

	if (minIndex == -1) {
		return o.base.Host
	} else {
		return o.base.RouterList[minIndex].host
	}
}

func (o *ImpedenceOffloader) forceEnq(req *http.Request) *list.Element {
	return o.base.forceEnq(req)
}

func (o *ImpedenceOffloader) preProxyMetric(req *http.Request, candidate string) interface{} {
	return time.Now()
}

func (o *ImpedenceOffloader) postProxyMetric(req *http.Request, candidate string, preProxyMetric interface{}) {
	// Asserting that preProxyMetric holds time.Time value.
	timeElapsed := time.Since(preProxyMetric.(time.Time))
	candidateIdx := o.candidateToIndex[candidate]

	prevRouterWeight := o.base.RouterList[candidateIdx].weight
	o.base.RouterList[candidateIdx].weight = prevRouterWeight * (1 - o.alpha) + float64(timeElapsed.Microseconds()/1000) * o.alpha
}

func (o *ImpedenceOffloader) Deq(req *http.Request, ctx *list.Element) {
	o.base.Deq(req, ctx)
}
func (o *ImpedenceOffloader) isOffloaded(req *http.Request) bool {
	return o.base.isOffloaded(req)
}
func (o *ImpedenceOffloader) getStatusStr() string {
	return o.base.getStatusStr()
}
