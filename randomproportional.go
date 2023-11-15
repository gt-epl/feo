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
	*BaseOffloader //hacky embedding, because you cannot override methods of an embedding
	alpha				float64
	randompropAlpha		float64
	randompropBeta		float64
	candidateToIndex 	map[string]int
	mu					sync.Mutex
	ExtendRouterList	[]extendRouter	
}

func NewRandomPropOffloader(base *BaseOffloader) *RandomPropOffloader {

	// NOTE: According to the serverlessonedge implementation, the alpha: 1, randompropAlpha: 1, randomPropBeta: 1
	// These numbers can be connfigured and compared if needed.

	randomPropOffloader := &RandomPropOffloader{alpha: 1.0, randompropAlpha: 0.99, randompropBeta: 0.99, BaseOffloader: base}
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

func (o *RandomPropOffloader) CheckAndEnq(req *http.Request) (*list.Element, bool) {
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

func (o *RandomPropOffloader) GetOffloadCandidate(req *http.Request) string {
	o.mu.Lock()
	defer o.mu.Unlock()

	total_nodes := len(o.RouterList)

	maxWeight := -1.0
	maxIndex := -1

	curTime := time.Now()

	for i:=0; i<total_nodes; i++ {
		latency := o.RouterList[i].weight
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
		return o.Host
	} else {
		return o.RouterList[maxIndex].host
	}
}

// func (o *RandomPropOffloader) PreProxyMetric(req *http.Request, candidate string) interface{} {
// 	return time.Now()
// }

// func (o *RandomPropOffloader) PostProxyMetric(req *http.Request, candidate string, preProxyMetric interface{}) {
// 	// Asserting that preProxyMetric holds time.Time value.
// 	timeElapsed := time.Since(preProxyMetric.(time.Time))
// 	candidateIdx := o.candidateToIndex[candidate]

// 	o.mu.Lock()
// 	defer o.mu.Unlock()

// 	prevRouterWeight := o.RouterList[candidateIdx].weight
// 	o.RouterList[candidateIdx].weight = prevRouterWeight * (1 - o.alpha) + float64(timeElapsed.Microseconds()/1000) * o.alpha

// 	o.ExtendRouterList[candidateIdx].lambdasServed += 1
// 	o.ExtendRouterList[candidateIdx].lastResponse = time.Now()
// }

func (o *RandomPropOffloader) MetricSMAnalyze(ctx *list.Element) {

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

			o.ExtendRouterList[candidateIdx].lambdasServed += 1
			o.ExtendRouterList[candidateIdx].lastResponse = time.Now()
		}
	}
}