package main

import (
	"container/list"
	"log"
	"net/http"
)

type RoundRobinOffloader struct {
	*BaseOffloader
	cur_idx int
}

func NewRoundRobinOffloader(base *BaseOffloader) *RoundRobinOffloader {
	return &RoundRobinOffloader{cur_idx: 0, BaseOffloader: base}
}

func (o *RoundRobinOffloader) CheckAndEnq(req *http.Request) (*list.Element, bool) {
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

func (o *RoundRobinOffloader) GetOffloadCandidate(req *http.Request) string {
	candidate := o.RouterList[o.cur_idx].host
	total_nodes := len(o.RouterList)
	o.cur_idx = (o.cur_idx + 1) % total_nodes
	return candidate
}