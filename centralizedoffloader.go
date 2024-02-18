package main

import (
	"container/list"
	"net/http"
	"time"
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

func (o *CentralizedOffloader) MetricSMAnalyze(ctx *list.Element) {

	var timeElapsed time.Duration
	if (ctx.Value.(*MetricSM).state == FinalState) && (ctx.Value.(*MetricSM).candidate != "default") {
		if ctx.Value.(*MetricSM).local {
			timeElapsed = ctx.Value.(*MetricSM).postLocal.Sub(ctx.Value.(*MetricSM).preLocal)
		} else {
			timeElapsed = ctx.Value.(*MetricSM).postOffload.Sub(ctx.Value.(*MetricSM).preOffload)
		}
		ctx.Value.(*MetricSM).elapsed = timeElapsed
	}
}
