package main

import (
	"container/list"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

type OffloadPolicy string

const (
	OffloadAlternate = "alternate"
	OffloadFederated = "federated"
	OffloadCentrak   = "central"
)

func OffloadFactory(pol OffloadPolicy, routerList []router, host string) OffloaderIntf {
	base := newBaseOffloader(host, routerList)
	switch pol {
	case OffloadAlternate:
		return &AlternateOffloader{local_flag: true, BaseOffloader: base}
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
	Deq(req *http.Request, ctx *list.Element)
	isOffloaded(req *http.Request) bool
	getOffloadCandidate(req *http.Request) string
	getStatusStr() string
}

type BaseOffloader struct {
	finfo      FunctionInfo
	host       string
	routerList []router
}

func newBaseOffloader(host string, routerList []router) *BaseOffloader {
	o := BaseOffloader{}
	o.finfo.invoke_list = list.New()
	return &o
}

func (o *BaseOffloader) checkAndEnq(req *http.Request) (*list.Element, bool) {
	return o.forceEnq(req), true
}

func (o *BaseOffloader) isOffloaded(req *http.Request) bool {
	forwardedField := req.Header.Get("X-Forwarded-For")
	return len(strings.TrimSpace(forwardedField)) != 0
}

func (o *BaseOffloader) forceEnq(req *http.Request) *list.Element {
	//template string (/api/v1/namespaces/guest/actions/copy)
	o.finfo.mu.Lock()
	defer o.finfo.mu.Unlock()
	o.finfo.name = strings.Split(req.URL.Path, "/")[5]
	ctx := o.finfo.invoke_list.PushBack(time.Now())
	return ctx
}

func (o *BaseOffloader) Deq(req *http.Request, ctx *list.Element) {
	o.finfo.mu.Lock()
	defer o.finfo.mu.Unlock()
	o.finfo.invoke_list.Remove(ctx)
}

func (o *BaseOffloader) getOffloadCandidate(req *http.Request) string {
	return ""
}

func (o *BaseOffloader) getStatusStr() string {
	snap := o.finfo.getSnapshot()
	snap.HasCapacity = false
	jbytes, _ := json.Marshal(snap)
	return string(jbytes)
}
