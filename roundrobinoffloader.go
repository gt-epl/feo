package main

import (
	"container/list"
	"log"
	"net/http"
)

type RoundRobinOffloader struct {
	base    *BaseOffloader //hacky embedding, because you cannot override methods of an embedding
	cur_idx int
}

func newRoundRobinOffloader(base *BaseOffloader) *RoundRobinOffloader {
	return &RoundRobinOffloader{cur_idx: 0, base: base}
}

func (o *RoundRobinOffloader) checkAndEnq(req *http.Request) (*list.Element, bool) {
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

func (o *RoundRobinOffloader) getOffloadCandidate(req *http.Request) string {
	candidate := o.base.RouterList[o.cur_idx].host
	total_nodes := len(o.base.RouterList)
	o.cur_idx = (o.cur_idx + 1) % total_nodes
	return candidate
}

func (o *RoundRobinOffloader) forceEnq(req *http.Request) *list.Element {
	return o.base.forceEnq(req)
}
func (o *RoundRobinOffloader) Deq(req *http.Request, ctx *list.Element) {
	o.base.Deq(req, ctx)
}
func (o *RoundRobinOffloader) isOffloaded(req *http.Request) bool {
	return o.base.isOffloaded(req)
}
func (o *RoundRobinOffloader) getStatusStr() string {
	return o.base.getStatusStr()
}
