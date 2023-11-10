package main

import (
	"container/list"
	"log"
	"math"
	"net/http"
	"sync"
	"time"
)

type extendRouter struct {
	routerInfo		router
	lambdasServed	int64
	lastResponse	time.Time
}

type RandomPropOffloader struct {
	base    			*BaseOffloader //hacky embedding, because you cannot override methods of an embedding
	alpha				float64
	randompropAlpha		float64
	randompropBeta		float64
	candidateToIndex 	map[string]int
	mu					sync.Mutex
	ExtendRouterList	[]extendRouter	
}

func newRandomPropOffloader(base *BaseOffloader) *RandomPropOffloader {

	// NOTE: According to the serverlessonedge implementation, the alpha: 1, randompropAlpha: 1, randomPropBeta: 1
	// These numbers can be connfigured and compared if needed.

	randomPropOffloader := &RandomPropOffloader{alpha: 1.0, randompropAlpha: 0.99, randompropBeta: 0.99, base: base}
	randomPropOffloader.candidateToIndex = make(map[string]int)

	curTime := time.Now()

	for idx,router := range base.RouterList {
		randomPropOffloader.candidateToIndex[router.host] = idx

		var newExtendRouter extendRouter
		newExtendRouter.routerInfo = router
		newExtendRouter.lambdasServed = 0
		newExtendRouter.lastResponse = curTime

		randomPropOffloader.ExtendRouterList = append(randomPropOffloader.ExtendRouterList, newExtendRouter)
	}

	return randomPropOffloader
}

func (o *RandomPropOffloader) checkAndEnq(req *http.Request) (*list.Element, bool) {
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

func (o *RandomPropOffloader) getOffloadCandidate(req *http.Request) string {
	o.mu.Lock()
	defer o.mu.Unlock()

	total_nodes := len(o.base.RouterList)

	maxWeight := -1.0
	maxIndex := -1

	curTime := time.Now()

	for i:=0; i<total_nodes; i++ {
		latency := o.base.RouterList[i].weight
		lambdasServed := o.ExtendRouterList[i].lambdasServed
		timeSinceLastResponse := float64(curTime.Sub(o.ExtendRouterList[i].lastResponse).Microseconds())/1000

		if latency == 0.0 {
			latency = 1.0
		}
		
		if lambdasServed == 0 {
			lambdasServed = 1
		}

		if timeSinceLastResponse == 0.0 {
			continue
		}

		curWeight := math.Pow(1/latency, o.randompropAlpha)/math.Pow((float64(lambdasServed)/timeSinceLastResponse), o.randompropBeta)
		if (maxWeight < curWeight) {
			maxIndex = i
			maxWeight = curWeight
		}
	}

	if (maxIndex == -1) {
		return o.base.Host
	} else {
		return o.base.RouterList[maxIndex].host
	}
}

func (o *RandomPropOffloader) forceEnq(req *http.Request) *list.Element {
	return o.base.forceEnq(req)
}

func (o *RandomPropOffloader) preProxyMetric(req *http.Request, candidate string) interface{} {
	return time.Now()
}

func (o *RandomPropOffloader) postProxyMetric(req *http.Request, candidate string, preProxyMetric interface{}) {
	// Asserting that preProxyMetric holds time.Time value.
	timeElapsed := time.Since(preProxyMetric.(time.Time))
	candidateIdx := o.candidateToIndex[candidate]

	o.mu.Lock()
	defer o.mu.Unlock()

	prevRouterWeight := o.base.RouterList[candidateIdx].weight
	o.base.RouterList[candidateIdx].weight = prevRouterWeight * (1 - o.alpha) + float64(timeElapsed.Microseconds()/1000) * o.alpha

	o.ExtendRouterList[candidateIdx].lambdasServed += 1
	o.ExtendRouterList[candidateIdx].lastResponse = time.Now()
}

func (o *RandomPropOffloader) Deq(req *http.Request, ctx *list.Element) {
	o.base.Deq(req, ctx)
}
func (o *RandomPropOffloader) isOffloaded(req *http.Request) bool {
	return o.base.isOffloaded(req)
}
func (o *RandomPropOffloader) getStatusStr() string {
	return o.base.getStatusStr()
}
