package main

import (
	"container/heap"
	"container/list"
	// "log"
	"math"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// The priority queue taken from go docs: https://pkg.go.dev/container/heap

type CacheElement struct {
	candidate		string
	weight			float64
	lastUpdated		time.Time
	active			bool
	deficit			float64
	stalePeriod		time.Duration
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
func (pq *PriorityQueue) update(item *Item, deficit	float64) {
	item.ce.deficit = deficit
	heap.Fix(pq, item.index)
}

type RoundRobinLatencyOffloader struct {
	*BaseOffloader //hacky embedding, because you cannot override methods of an embedding
	
	mu					sync.Mutex

	alpha				float64
	
	candidateToItem		map[string]*Item
	pq					PriorityQueue
	itemArray			[]*Item					

	initStalePeriod		float64
	backoffCoefficient	float64
	maxStalePeriod		float64

	//util
	randUtil			*rand.Rand
}

func NewCacheElement(weight float64, deficit float64, initStalePeriod float64, hostName string) *CacheElement {
	cacheElement := &CacheElement{weight: weight, deficit: deficit, stalePeriod: time.Duration(initStalePeriod) * time.Second, candidate: hostName}
	cacheElement.lastUpdated = time.Now()
	cacheElement.active = true
	cacheElement.probing = false

	return cacheElement
}

func (o *CacheElement) ResetStalePeriod(initStalePeriod float64) {
	o.stalePeriod = time.Duration(initStalePeriod) * time.Second
}

func (o *CacheElement) UpdateStalePeriod(maxStalePeriod float64, backoffCoefficient float64) {

	tempStalePeriod := time.Duration(backoffCoefficient) * o.stalePeriod
	thresPeriod := time.Duration(maxStalePeriod) * time.Second
	if (thresPeriod > tempStalePeriod) {
		o.stalePeriod = tempStalePeriod
	} else{
		o.stalePeriod = thresPeriod
	}
}

func NewRoundRobinLatencyOffloader(base *BaseOffloader) *RoundRobinLatencyOffloader {

	// NOTE: According to the serverlessonedge implementation, the alpha: 1, randompropAlpha: 1, randomPropBeta: 1
	// These numbers can be connfigured and compared if needed.

	rrLatencyOffloader := &RoundRobinLatencyOffloader{alpha: 0.2, initStalePeriod: 1.0, backoffCoefficient: 2.0, maxStalePeriod: 30.0, BaseOffloader: base}
	// rrLatencyOffloader.candidateToIndex = make(map[string]int)

	rrLatencyOffloader.candidateToItem = make(map[string]*Item)
	rrLatencyOffloader.itemArray = make([]*Item, 0)

	rrLatencyOffloader.pq = make(PriorityQueue, 0)
	heap.Init(&rrLatencyOffloader.pq)

	for idx,router := range base.RouterList {
		// rrLatencyOffloader.candidateToIndex[router.host] = idx

		newCacheElement := NewCacheElement(0.0, 0.0, rrLatencyOffloader.initStalePeriod, router.host)
		newItem := &Item{ce: newCacheElement, index: idx}

		rrLatencyOffloader.candidateToItem[router.host] = newItem
		rrLatencyOffloader.itemArray = append(rrLatencyOffloader.itemArray, newItem)

		heap.Push(&rrLatencyOffloader.pq, newItem)
	}
	
	// Src: https://gobyexample.com/random-numbers
	s1 := rand.NewSource(time.Now().UnixNano())
    rrLatencyOffloader.randUtil = rand.New(s1)

	return rrLatencyOffloader
}

func (o *RoundRobinLatencyOffloader) GetLowestWeightItem() *Item {

	lowestWtItem := o.pq[0]

	for _,item := range o.pq {
		if lowestWtItem.ce.weight > item.ce.weight {
			lowestWtItem = item
		}
	}

	return lowestWtItem
}

func (o *RoundRobinLatencyOffloader) GetOffloadCandidate(req *http.Request) string {

	total_nodes := len(o.itemArray)

	curTime := time.Now()

	o.mu.Lock()
	defer o.mu.Unlock()

	var indexArr []int
	// O(n) -> n is the number of nodes.
	for i:=0; i<total_nodes; i++ {
		if (!o.itemArray[i].ce.active && !o.itemArray[i].ce.probing && curTime.After(o.itemArray[i].ce.lastUpdated.Add(o.itemArray[i].ce.stalePeriod))) {
			indexArr = append(indexArr, i)
		}
	}

	idx := -1
	if (len(indexArr) != 0) {
		idx = indexArr[o.randUtil.Intn(len(indexArr))]
		o.itemArray[idx].ce.probing = true
	} else {
		if (o.pq[len(o.pq)-1].ce.active) {
			idx = len(o.pq)-1

			o.pq.update(o.pq[idx], o.pq[idx].ce.deficit + o.pq[idx].ce.weight)
		}
	}

	if (idx == -1) {
		return o.Host
	} else {
		return o.pq[idx].ce.candidate
	}
}

func (o *RoundRobinLatencyOffloader) MetricSMAnalyze(ctx *list.Element) {

	var timeElapsed time.Duration
	if ((ctx.Value.(*MetricSM).state == FinalState) && (ctx.Value.(*MetricSM).candidate != "default")) {
		if (ctx.Value.(*MetricSM).local) {
			timeElapsed = ctx.Value.(*MetricSM).postLocal.Sub(ctx.Value.(*MetricSM).preLocal)
		} else {
			timeElapsed = ctx.Value.(*MetricSM).postOffload.Sub(ctx.Value.(*MetricSM).preOffload)
		}

		o.mu.Lock()
		defer o.mu.Unlock()

		candidateItem := o.candidateToItem[ctx.Value.(*MetricSM).candidate]

		// o.ExtendRouterList[candidateIdx].lambdasServed += 1
		// o.ExtendRouterList[candidateIdx].lastResponse = time.Now()
		
		if (candidateItem.ce.probing) {
			candidateItem.ce.probing = false
			
			// Gets the lowest weight item from the active set.
			// O(n) -> where n is the number of routers
			lowestWtItem := o.GetLowestWeightItem()

			if (float64(timeElapsed.Microseconds()/1000) <= 2 * lowestWtItem.ce.weight) {
				lowestDeficit := o.pq[len(o.pq)-1].ce.deficit

				// O(nlogn) -> where n is the number of routers
				for _,item := range o.itemArray {
					if (item.ce.active) {
						o.pq.update(item, item.ce.deficit - lowestDeficit)
					}
				}
				candidateItem.ce.deficit = float64(timeElapsed.Microseconds()/1000)
				candidateItem.ce.weight = float64(timeElapsed.Microseconds()/1000)
				candidateItem.ce.ResetStalePeriod(o.initStalePeriod)

				heap.Push(&o.pq, candidateItem)
			} else {
				candidateItem.ce.UpdateStalePeriod(o.maxStalePeriod, o.backoffCoefficient)
			}
		} else {
			prevRouterWeight := candidateItem.ce.weight
			candidateItem.ce.weight = prevRouterWeight * (1 - o.alpha) + float64(timeElapsed.Microseconds()/1000) * o.alpha
			
			// O(n) -> where n is the number of routers
			lowestWtItem := o.GetLowestWeightItem()
			
			if (candidateItem.ce.weight > 2*lowestWtItem.ce.weight) {
				candidateItem.ce.active = false
				o.pq.update(candidateItem, -1)
				heap.Pop(&o.pq)
			}
		}
		candidateItem.ce.lastUpdated = time.Now()
	} else {
		// The call failed, we need to do some cleanup so that this router can be picked again and tried.

		// ADDED based on intuition.

		if (ctx.Value.(*MetricSM).candidate != "default") {

			o.mu.Lock()
			defer o.mu.Unlock()

			candidateItem := o.candidateToItem[ctx.Value.(*MetricSM).candidate]

			if (candidateItem.ce.probing) {
				candidateItem.ce.probing = false
			} else {
				// If the candidate is still active, we subtract the current weight (possibly not the weight that was added) to the deficit.

				if (candidateItem.ce.active) {
					o.pq.update(candidateItem, math.Max(candidateItem.ce.deficit - candidateItem.ce.weight, 0.0))
				}
			}
		}
	}
}