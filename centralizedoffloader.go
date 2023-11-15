package main

import (
	"container/list"
	"net/http"
)

type CentralizedOffloader struct {
	*HybridOffloader
}

func NewCentralizedOffloader(base *BaseOffloader) *CentralizedOffloader {

	fed := &CentralizedOffloader{}
	fed.HybridOffloader = NewHybridOffloader(base)
	return fed
}

func (o *CentralizedOffloader) CheckAndEnq(req *http.Request) (*list.Element, bool) {
	//centralized always offloads
	var ele *list.Element
	status := false

	if o.IsOffloaded(req) {
		ele, status = o.BaseOffloader.CheckAndEnq(req)
	}
	return ele, status
}
