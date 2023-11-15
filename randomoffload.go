package main

import (
	"container/list"
	"log"
	"net/http"

	// Should we use crypto/rand instead? Latency will probably be higher.
	"math/rand"
)

type RandomOffloader struct {
	*BaseOffloader //hacky embedding, because you cannot override methods of an embedding
	cur_idx        int
}

func NewRandomOffloader(base *BaseOffloader) *RandomOffloader {
	return &RandomOffloader{cur_idx: 0, BaseOffloader: base}
}

func (o *RandomOffloader) CheckAndEnq(req *http.Request) (*list.Element, bool) {
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

func (o *RandomOffloader) getOffloadCandidate(req *http.Request) string {
	total_nodes := len(o.RouterList)
	o.cur_idx = rand.Intn(100) % total_nodes
	candidate := o.RouterList[o.cur_idx].host
	return candidate
}

func (o *RandomOffloader) forceEnq(req *http.Request) *list.Element {
	return o.base.forceEnq(req)
}
func (o *RandomOffloader) preProxyMetric(req *http.Request, candidate string) interface{} {
	return o.preProxyMetric(req, candidate)
}
func (o *RandomOffloader) postProxyMetric(req *http.Request, candidate string, preProxyMetric interface{}) {
	o.postProxyMetric(req, candidate, preProxyMetric)
}
func (o *RandomOffloader) Deq(req *http.Request, ctx *list.Element) {
	o.base.Deq(req, ctx)
}
func (o *RandomOffloader) isOffloaded(req *http.Request) bool {
	return o.base.isOffloaded(req)
}
func (o *RandomOffloader) getStatusStr() string {
	return o.base.getStatusStr()
}
