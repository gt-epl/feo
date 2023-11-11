package main

import (
	"container/list"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

type OffloadPolicy string

const (
	OffloadRoundRobin = "roundrobin"
	OffloadRandom     = "random"
	OffloadFederated  = "federated"
	OffloadCentral    = "central"
	OffloadHybrid     = "hybrid"
)

func OffloadFactory(pol OffloadPolicy, routerList []router, host string) OffloaderIntf {
	base := NewBaseOffloader(host, routerList)
	switch pol {
	case OffloadRoundRobin:
		log.Println("[INFO] Selecting RoundRobin Offloader")
		return NewRoundRobinOffloader(base)
	case OffloadRandom:
		log.Println("[INFO] Selecting Random Offloader")
		return NewRandomOffloader(base)
	case OffloadFederated:
		log.Println("[INFO] Selecting Federated Offloader")
		return NewFederatedOffloader(base)
	case OffloadHybrid:
		log.Println("[INFO] Selecting Federated Offloader")
		return NewHybridOffloader(base)
	default:
		log.Println("[WARNING] No policy specified. Selecting Base Offloader")
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
	defer f.mu.Unlock()
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
	CheckAndEnq(req *http.Request) (*list.Element, bool)
	ForceEnq(req *http.Request) *list.Element
	Deq(req *http.Request, ctx *list.Element)
	IsOffloaded(req *http.Request) bool
	GetOffloadCandidate(req *http.Request) string
	GetStatusStr() string
	Close()
}

// TODO: This is single function currently. Provide multi-function support.
type BaseOffloader struct {
	Finfo          FunctionInfo
	Host           string
	RouterList     []router
	Qlen_max       int32
	ControllerAddr string
}

func NewBaseOffloader(host string, routerList []router) *BaseOffloader {
	o := BaseOffloader{Host: host, RouterList: routerList, Qlen_max: math.MaxInt32}
	o.Finfo.invoke_list = list.New()
	return &o
}

func (o *BaseOffloader) CheckAndEnq(req *http.Request) (*list.Element, bool) {
	log.Println("[DEBUG] in checkAndEnq")
	o.Finfo.mu.Lock()
	defer o.Finfo.mu.Unlock()
	log.Println("[DEBUG ]qlen: ", o.Finfo.invoke_list.Len())
	if o.Finfo.invoke_list.Len() < int(o.Qlen_max) {
		return o.Finfo.invoke_list.PushBack(time.Now()), true
	} else {
		return nil, false
	}
}

func (o *BaseOffloader) IsOffloaded(req *http.Request) bool {
	forwardedField := req.Header.Get("X-Offloaded-For")
	log.Println("[DEBUG]isOffloaded ", forwardedField)
	return len(strings.TrimSpace(forwardedField)) != 0
}

func (o *BaseOffloader) ForceEnq(req *http.Request) *list.Element {
	//template string (/api/v1/namespaces/guest/actions/copy)
	o.Finfo.mu.Lock()
	defer o.Finfo.mu.Unlock()
	o.Finfo.name = strings.Split(req.URL.Path, "/")[5]
	ctx := o.Finfo.invoke_list.PushBack(time.Now())
	return ctx
}

func (o *BaseOffloader) Deq(req *http.Request, ctx *list.Element) {
	o.Finfo.mu.Lock()
	defer o.Finfo.mu.Unlock()
	o.Finfo.invoke_list.Remove(ctx)
}

func (o *BaseOffloader) GetOffloadCandidate(req *http.Request) string {
	return ""
}

func (o *BaseOffloader) GetStatusStr() string {
	log.Println("[DEBUG] in getStatusStr")
	snap := o.Finfo.getSnapshot()
	snap.HasCapacity = false
	jbytes, err := json.Marshal(snap)
	if err != nil {
		log.Println("[WARNING] could not marshall status: ", err)
	}
	return string(jbytes)
}

func (o *BaseOffloader) Close() {

}