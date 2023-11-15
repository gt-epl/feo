package main

import (
	"container/heap"
	// "container/list"
	// "log"
	"math"
	// "net/http"
	"sync"
	// "time"
)

// The priority queue taken from go docs: https://pkg.go.dev/container/heap

type CacheElement struct {
	weight			float64
	lastUpdated		float64
	removed			bool
	deficit			float64
	stalePeriod		float64
	probing			bool
}

type Item struct {
	ce				*CacheElement
	index 			int
}
type PriorityQueue []*Item

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// We want Pop to give us the highest, not lowest, priority so we use greater than here.
	return pq[i].ce.deficit < pq[j].ce.deficit
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x any) {
	n := len(*pq)
	item := x.(*Item)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

// update modifies the priority and value of an Item in the queue.
func (pq *PriorityQueue) update(item *Item, deficit	float64, priority int) {
	item.ce.deficit = deficit
	heap.Fix(pq, item.index)
}

type RoundRobinLatencyOffloader struct {
	*BaseOffloader //hacky embedding, because you cannot override methods of an embedding
	
	candidateToIndex 	map[string]int
	mu					sync.Mutex
	
	hostToCE			map[string]*CacheElement
	pq					PriorityQueue
	initStalePeriod		float64
	backoffCoefficient	float64
	maxStalePeriod		float64
}

func NewCacheElement(weight float64, deficit float64, initStalePeriod float64) *CacheElement {
	cacheElement := &CacheElement{weight: weight, deficit: deficit, stalePeriod: initStalePeriod}
	cacheElement.lastUpdated = -1
	cacheElement.removed = false
	cacheElement.probing = false

	return cacheElement
}

func (o *CacheElement) ResetStalePeriod(initStalePeriod float64) {
	o.stalePeriod = initStalePeriod
}

func (o *CacheElement) UpdateStalePeriod(maxStalePeriod float64, backoffCoefficient float64) {
	o.stalePeriod = math.Min(maxStalePeriod, backoffCoefficient * o.stalePeriod)
}

func NewRoundRobinLatencyOffloader(base *BaseOffloader) *RoundRobinLatencyOffloader {

	// NOTE: According to the serverlessonedge implementation, the alpha: 1, randompropAlpha: 1, randomPropBeta: 1
	// These numbers can be connfigured and compared if needed.

	rrLatencyOffloader := &RoundRobinLatencyOffloader{initStalePeriod: 1.0, backoffCoefficient: 2.0, maxStalePeriod: 30.0, BaseOffloader: base}
	rrLatencyOffloader.candidateToIndex = make(map[string]int)

	rrLatencyOffloader.hostToCE = make(map[string]*CacheElement)
	rrLatencyOffloader.pq = make(PriorityQueue, 0)
	heap.Init(&rrLatencyOffloader.pq)

	for idx,router := range base.RouterList {
		rrLatencyOffloader.candidateToIndex[router.host] = idx
	}

	return rrLatencyOffloader
}

// func (o *RoundRobinLatencyOffloader) CheckAndEnq(req *http.Request) (*list.Element, bool) {
// 	//NOTE: in round robin checkAndEnq returns true AND does a forceenq IF it is an offloaded request.
// 	// var ctx *list.Element
// 	// enq_success := false
// 	// if o.IsOffloaded(req) {
// 	// 	log.Println("[INFO] Already offloaded. Force Enq")
// 	// 	ctx = o.ForceEnq(req)
// 	// 	enq_success = true
// 	// }
// 	// return ctx, enq_success
// }

// func (o *RoundRobinLatencyOffloader) GetOffloadCandidate(req *http.Request) string {
// 	// o.mu.Lock()
// 	// defer o.mu.Unlock()

// 	// total_nodes := len(o.RouterList)

// 	// maxWeight := -1.0
// 	// maxIndex := -1

// 	// curTime := time.Now()

// 	// for i:=0; i<total_nodes; i++ {
// 	// 	latency := o.RouterList[i].weight
// 	// 	lambdasServed := o.ExtendRouterList[i].lambdasServed
// 	// 	timeSinceLastResponse := float64(curTime.Sub(o.ExtendRouterList[i].lastResponse).Microseconds())/1000

// 	// 	if latency == 0.0 {
// 	// 		latency = 1.0
// 	// 	}
		
// 	// 	if lambdasServed == 0 {
// 	// 		lambdasServed = 1
// 	// 	}

// 	// 	if timeSinceLastResponse == 0.0 {
// 	// 		continue
// 	// 	}

// 	// 	curWeight := math.Pow(1/latency, o.randompropAlpha)/math.Pow((float64(lambdasServed)/timeSinceLastResponse), o.randompropBeta)
// 	// 	if (maxWeight < curWeight) {
// 	// 		maxIndex = i
// 	// 		maxWeight = curWeight
// 	// 	}
// 	// }

// 	// if (maxIndex == -1) {
// 	// 	return o.Host
// 	// } else {
// 	// 	return o.RouterList[maxIndex].host
// 	// }
// }

// func (o *RoundRobinLatencyOffloader) PreProxyMetric(req *http.Request, candidate string) interface{} {
// 	// return time.Now()
// }

// func (o *RoundRobinLatencyOffloader) PostProxyMetric(req *http.Request, candidate string, preProxyMetric interface{}) {
// 	// Asserting that preProxyMetric holds time.Time value.
// 	// timeElapsed := time.Since(preProxyMetric.(time.Time))
// 	// candidateIdx := o.candidateToIndex[candidate]

// 	// o.mu.Lock()
// 	// defer o.mu.Unlock()

// 	// prevRouterWeight := o.RouterList[candidateIdx].weight
// 	// o.RouterList[candidateIdx].weight = prevRouterWeight * (1 - o.alpha) + float64(timeElapsed.Microseconds()/1000) * o.alpha

// 	// o.ExtendRouterList[candidateIdx].lambdasServed += 1
// 	// o.ExtendRouterList[candidateIdx].lastResponse = time.Now()
// }