package main

import (
	"container/list"
	"log"
	"net/http"

	// Should we use crypto/rand instead? Latency will probably be higher.
	"math/rand"
)

type FederatedOffloader struct {
	base    *BaseOffloader //hacky embedding, because you cannot override methods of an embedding
	cur_idx int
}

func newFederatedOffloader(base *BaseOffloader) *FederatedOffloader {
	base.Qlen_max = 2
	fed := &FederatedOffloader{cur_idx: 0, base: base}
	log.Println("[DEBUG] qlen_max = ", fed.base.Qlen_max)
	return fed
}

func (o *FederatedOffloader) checkAndEnq(req *http.Request) (*list.Element, bool) {
	//NOTE: Qlen_max has been set during initialization
	return o.base.checkAndEnq(req)
}

func (o *FederatedOffloader) getOffloadCandidate(req *http.Request) string {
	total_nodes := len(o.base.RouterList)
	o.cur_idx = rand.Intn(100) % total_nodes
	candidate := o.base.RouterList[o.cur_idx].host
	return candidate
}

func (o *FederatedOffloader) forceEnq(req *http.Request) *list.Element {
	return o.base.forceEnq(req)
}
func (o *FederatedOffloader) Deq(req *http.Request, ctx *list.Element) {
	o.base.Deq(req, ctx)
}
func (o *FederatedOffloader) isOffloaded(req *http.Request) bool {
	return o.base.isOffloaded(req)
}
func (o *FederatedOffloader) getStatusStr() string {
	return o.base.getStatusStr()
}
