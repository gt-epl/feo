package main

import (
	"log"
	"math/rand"
	"net/http"
)

type FederatedOffloader struct {
	*BaseOffloader
	cur_idx int
}

func NewFederatedOffloader(base *BaseOffloader) *FederatedOffloader {
	fed := &FederatedOffloader{cur_idx: 0, BaseOffloader: base}
	fed.Qlen_max = 2
	log.Println("[DEBUG] qlen_max = ", fed.Qlen_max)
	return fed
}

func (o *FederatedOffloader) GetOffloadCandidate(req *http.Request) string {
	total_nodes := len(o.RouterList)
	//NOTE: you do not want to include the first node which is self
	o.cur_idx = 1 + rand.Intn(100)%(total_nodes-1)
	candidate := o.RouterList[o.cur_idx].host
	return candidate
}
