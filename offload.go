package main

import (
	"container/list"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type OffloadPolicy string

const (
	OffloadRoundRobin = "roundrobin"
	OffloadRandom = "random"
	OffloadFederated  = "federated"
	OffloadCentrak    = "central"
	OffloadImpedence = "impedence"
	RandomProportional = "randomproportional"
)

func OffloadFactory(pol OffloadPolicy, routerList []router, host string) OffloaderIntf {
	base := newBaseOffloader(host, routerList)
	switch pol {
	case OffloadRoundRobin:
		return newRoundRobinOffloader(base)
	case OffloadRandom:
		return newRandomOffloader(base)
	case OffloadImpedence:
		return newImpedenceOffloader(base)
	case RandomProportional:
		return newRandomPropOffloader(base)
	default:
		return base
	}
}

type FunctionInfo struct {
	name        string
	invoke_list *list.List
	mu          sync.Mutex
}

func (f *FunctionInfo) getSnapshot() Snapshot {
	f.mu.Lock()
	defer f.mu.Lock()
	return Snapshot{
		Name: f.name,
		Qlen: f.invoke_list.Len(),
	}
}

type Snapshot struct {
	Name        string `json:"name"`
	Qlen        int    `json:"qlen"`
	HasCapacity bool   `json:"hascapacity"`
}

type OffloaderIntf interface {
	checkAndEnq(req *http.Request) (*list.Element, bool)
	forceEnq(req *http.Request) *list.Element
	preProxyMetric(req *http.Request, candidate string) interface{}
	postProxyMetric(req* http.Request, candidate string, preProxyMetric interface{})
	Deq(req *http.Request, ctx *list.Element)
	isOffloaded(req *http.Request) bool
	getOffloadCandidate(req *http.Request) string
	getStatusStr() string
}

type BaseOffloader struct {
	Finfo      FunctionInfo
	Host       string
	RouterList []router
}

func newBaseOffloader(host string, routerList []router) *BaseOffloader {
	o := BaseOffloader{Host: host, RouterList: routerList}
	o.Finfo.invoke_list = list.New()
	return &o
}

func (o *BaseOffloader) checkAndEnq(req *http.Request) (*list.Element, bool) {
	return o.forceEnq(req), true
}

func (o *BaseOffloader) isOffloaded(req *http.Request) bool {
	forwardedField := req.Header.Get("X-Offloaded-For")
	log.Println("[DEBUG]isOffloaded ", forwardedField)
	return len(strings.TrimSpace(forwardedField)) != 0
}

func (o *BaseOffloader) forceEnq(req *http.Request) *list.Element {
	//template string (/api/v1/namespaces/guest/actions/copy)
	o.Finfo.mu.Lock()
	defer o.Finfo.mu.Unlock()
	o.Finfo.name = strings.Split(req.URL.Path, "/")[5]
	ctx := o.Finfo.invoke_list.PushBack(time.Now())
	return ctx
}

func (o *BaseOffloader) preProxyMetric(req *http.Request, candidate string) interface{} {
	return nil
}

func (o *BaseOffloader) postProxyMetric(req *http.Request, candidate string, preProxyMetric interface{}) {
	return
}

func (o *BaseOffloader) Deq(req *http.Request, ctx *list.Element) {
	o.Finfo.mu.Lock()
	defer o.Finfo.mu.Unlock()
	o.Finfo.invoke_list.Remove(ctx)
}

func (o *BaseOffloader) getOffloadCandidate(req *http.Request) string {
	return ""
}

func (o *BaseOffloader) getStatusStr() string {
	snap := o.Finfo.getSnapshot()
	snap.HasCapacity = false
	jbytes, _ := json.Marshal(snap)
	return string(jbytes)
}
