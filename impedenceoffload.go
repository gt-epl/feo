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
	routerInfo    router
	lambdasServed int64
	lastResponse  time.Time
	weight        float64
}

type ImpedenceOffloader struct {
	*BaseOffloader   //hacky embedding, because you cannot override methods of an embedding
	alpha            float64
	candidateToIndex map[string]int
	mu               sync.Mutex
	ExtendRouterList []extendRouter
}

func NewImpedenceOffloader(base *BaseOffloader) *ImpedenceOffloader {
	impedenceOffloader := &ImpedenceOffloader{alpha: 0.2, BaseOffloader: base}
	impedenceOffloader.candidateToIndex = make(map[string]int)

	for idx, router := range base.RouterList {
		impedenceOffloader.candidateToIndex[router.host] = idx

		var newExtendRouter extendRouter
		newExtendRouter.weight = 0.0
		newExtendRouter.routerInfo = router

		impedenceOffloader.ExtendRouterList = append(impedenceOffloader.ExtendRouterList, newExtendRouter)
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

	total_nodes := len(o.ExtendRouterList)

	minWeight := math.MaxFloat64
	minIndex := -1

	for i := 0; i < total_nodes; i++ {
		if minWeight > o.ExtendRouterList[i].weight {
			minIndex = i
			minWeight = o.ExtendRouterList[i].weight
		}
	}

	if minIndex == -1 {
		return o.Host
	} else {
		return o.ExtendRouterList[minIndex].routerInfo.host
	}
}

func (o *ImpedenceOffloader) MetricSMAnalyze(ctx *list.Element) {

	var timeElapsed time.Duration
	if (ctx.Value.(*MetricSM).state == FinalState) && (ctx.Value.(*MetricSM).candidate != "default") {
		if ctx.Value.(*MetricSM).local {
			timeElapsed = ctx.Value.(*MetricSM).postLocal.Sub(ctx.Value.(*MetricSM).preLocal)
		} else {
			timeElapsed = ctx.Value.(*MetricSM).postOffload.Sub(ctx.Value.(*MetricSM).preOffload)
		}
		ctx.Value.(*MetricSM).elapsed = timeElapsed

		if ctx.Value.(*MetricSM).localAfterFail || ctx.Value.(*MetricSM).localByDefault {
			log.Println("[DEBUG] Local candidate not chosen by GetOffloadCandidate, no further analysis.")
			return
		}

		if ctx.Value.(*MetricSM).candidate != "default" {
			candidateIdx := o.candidateToIndex[ctx.Value.(*MetricSM).candidate]

			o.mu.Lock()
			defer o.mu.Unlock()

			prevRouterWeight := o.ExtendRouterList[candidateIdx].weight
			o.ExtendRouterList[candidateIdx].weight = prevRouterWeight*(1-o.alpha) + float64(timeElapsed.Microseconds()/1000)*o.alpha
		}
	}
}
