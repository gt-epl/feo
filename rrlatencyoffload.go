package main

import (
	"container/heap"
	"container/list"
	"log"
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
	return pq[i].ce.deficit > pq[j].ce.deficit
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
	heap.Fix(pq, item.index)
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

func NewCacheElement(weight float64, deficit float64, initStalePeriod float64, hostName string) *CacheElement {
	cacheElement := &CacheElement{weight: weight, deficit: deficit, stalePeriod: time.Duration(initStalePeriod) * time.Second, candidate: hostName}
	cacheElement.lastUpdated = time.Now()
	cacheElement.active = true
	cacheElement.probing = false

	return cacheElement
}

func (c *CacheElement) ResetStalePeriod(initStalePeriod float64) {
	c.stalePeriod = time.Duration(initStalePeriod) * time.Second
}

func (c *CacheElement) UpdateStalePeriod(maxStalePeriod float64, backoffCoefficient float64) {

	tempStalePeriod := time.Duration(backoffCoefficient) * c.stalePeriod
	thresPeriod := time.Duration(maxStalePeriod) * time.Second
	if (thresPeriod > tempStalePeriod) {
		c.stalePeriod = tempStalePeriod
	} else{
		c.stalePeriod = thresPeriod
	}
}

type RRLatencyOffloader struct {
	*BaseOffloader //hacky embedding, because you cannot override methods of an embedding
	mu					sync.Mutex
	alpha				float64
	candidateToItem		map[string]*Item
	pq					PriorityQueue
	itemArray			[]*Item					
	initStalePeriod		float64
	backoffCoefficient	float64
	maxStalePeriod		float64
	randUtil			*rand.Rand
}

func NewRRLatencyOffloader(base *BaseOffloader) *RRLatencyOffloader {
	rrLatencyOffloader := &RRLatencyOffloader{BaseOffloader: base, alpha: 0.2, initStalePeriod: 1.0, backoffCoefficient: 2.0, maxStalePeriod: 30.0}
	// rrLatencyOffloader.candidateToIndex = make(map[string]int)

	// NOTE: According to the serverlessonedge implementation, the alpha: 1, randompropAlpha: 1, randomPropBeta: 1
	// These numbers can be connfigured and compared if needed.

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

		// heap.Push(&rrLatencyOffloader.pq, newItem)
		rrLatencyOffloader.pq.Push(newItem)
	}
	
	// Src: https://gobyexample.com/random-numbers
	s1 := rand.NewSource(time.Now().UnixNano())
    rrLatencyOffloader.randUtil = rand.New(s1)

	return rrLatencyOffloader
}

func (o *RRLatencyOffloader) CheckAndEnq(req *http.Request) (*list.Element, bool) {
	//NOTE: in round robin latency checkAndEnq returns true AND does a forceenq IF it is an offloaded request.
	var ctx *list.Element
	enq_success := false
	if o.IsOffloaded(req) {
		log.Println("[INFO] Already offloaded. Force Enq")
		ctx = o.ForceEnq(req)
		enq_success = true
	}
	return ctx, enq_success
}

func (o *RRLatencyOffloader) GetOffloadCandidate(req *http.Request) string {
	log.Println("[INFO] Selecting Candidate.")
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
		// fmt.Println("Choosing random router to probe.")
		idx = indexArr[o.randUtil.Intn(len(indexArr))]
		o.itemArray[idx].ce.probing = true

		return o.itemArray[idx].ce.candidate
	} else {
		if (o.pq[len(o.pq)-1].ce.active) {
			// log.Println("[DEBUG] Lowest weight:", o.pq[len(o.pq)-1].ce.weight)
			// log.Println("[DEBUG] Priority queue", o.pq[0].ce.weight)
			idx = len(o.pq)-1

			o.pq.update(o.pq[idx], o.pq[idx].ce.deficit + o.pq[idx].ce.weight)

			return o.pq[idx].ce.candidate
		} else {
			log.Println("[DEBUG] Candidate selection failed, returning host.")
			return o.Host
		}
	}
}

func (o *RRLatencyOffloader) MetricSMAnalyze(ctx *list.Element) {
	log.Println("[INFO] Analyzing Metrics.")
	var timeElapsed time.Duration
	if ((ctx.Value.(*MetricSM).state == FinalState) && (ctx.Value.(*MetricSM).candidate != "default")) {
		if (ctx.Value.(*MetricSM).local) {
			timeElapsed = ctx.Value.(*MetricSM).postLocal.Sub(ctx.Value.(*MetricSM).preLocal)
		} else {
			timeElapsed = ctx.Value.(*MetricSM).postOffload.Sub(ctx.Value.(*MetricSM).preOffload)
		}
		ctx.Value.(*MetricSM).elapsed = timeElapsed

		if (ctx.Value.(*MetricSM).localAfterFail || ctx.Value.(*MetricSM).localByDefault) {
			log.Println("[DEBUG] Local candidate not chosen by GetOffloadCandidate, no further analysis.")
			return
		}

		candidateItem := o.candidateToItem[ctx.Value.(*MetricSM).candidate]

		o.mu.Lock()
		defer o.mu.Unlock()

		// o.ExtendRouterList[candidateIdx].lambdasServed += 1
		// o.ExtendRouterList[candidateIdx].lastResponse = time.Now()
		
		if (candidateItem.ce.probing) {
			candidateItem.ce.probing = false
			
			// Gets the lowest weight item from the active set.
			// O(n) -> where n is the number of routers
			lowestWtItem := o.lowestWeightItem()
			// lowestWtItem := o.pq[0]

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
				candidateItem.ce.active = true

				// heap.Push(&o.pq, candidateItem)
				o.pq.Push(candidateItem)
			} else {
				candidateItem.ce.UpdateStalePeriod(o.maxStalePeriod, o.backoffCoefficient)
			}
		} else {
			prevRouterWeight := candidateItem.ce.weight
			candidateItem.ce.weight = prevRouterWeight * (1 - o.alpha) + float64(timeElapsed.Microseconds()/1000) * o.alpha
			
			if (candidateItem.ce.active) {

				// ADDED: Check if this makes sense.
				// It only makes sense to do this if the node itself is active. Multiple requests could have gone through when active and it might have been probed and then set back to not-probe.

				// O(n) -> where n is the number of routers
				lowestWtItem := o.lowestWeightItem()
				// lowestWtItem := o.pq[0]

				if (candidateItem.ce.weight > 2*lowestWtItem.ce.weight) {
					candidateItem.ce.active = false

					// for i := 0; i<len(o.pq); i++ {
					// 	log.Println("[DEBUG] deficit value", o.pq[i].ce.deficit)
					// }
					// log.Println("Step 1")

					o.pq.update(candidateItem, -1)

					// for i := 0; i<len(o.pq); i++ {
					// 	log.Println("[DEBUG] deficit value", o.pq[i].ce.deficit)
					// }
					// log.Println("Step 2")

					// popItem := heap.Pop(&o.pq)
					popItem := o.pq.Pop()

					// for i := 0; i<len(o.pq); i++ {
					// 	log.Println("[DEBUG] deficit value", o.pq[i].ce.deficit)
					// }
					// log.Println("Step 3")

					if (popItem.(*Item) != candidateItem) {
						log.Println("[DEBUG] Popped item not equal to candidate item")
					}
				}
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

func (o *RRLatencyOffloader) lowestWeightItem() *Item {

	lowestWtItem := o.pq[0]

	for _,item := range o.pq {
		if lowestWtItem.ce.weight > item.ce.weight {
			lowestWtItem = item
		}
	}

	return lowestWtItem
}