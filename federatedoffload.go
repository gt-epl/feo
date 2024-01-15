package main

import (
	"container/list"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/mroth/weightedrand"
)

type FederatedOffloader struct {
	*BaseOffloader
	cur_idx int

	qlenMap map[string]float32
	mapMu   sync.Mutex
	qlen    float32
	qlenMu  sync.RWMutex
	wg      sync.WaitGroup
	quit    chan bool
}

func NewFederatedOffloader(base *BaseOffloader) *FederatedOffloader {
	fed := &FederatedOffloader{cur_idx: 0, BaseOffloader: base}
	fed.Qlen_max = 2
	log.Println("[DEBUG] qlen_max = ", fed.Qlen_max)
	fed.qlenMap = make(map[string]float32)
	for _, r := range fed.RouterList {
		fed.qlenMap[r.host] = 0
	}

	fed.wg.Add(1)
	//go fed.stateUpdateRoutine()
	return fed
}

func (o *FederatedOffloader) update_qlen() {
	defer o.wg.Done()

	cur_qlen := o.Finfo.getSnapshot().Qlen

	o.qlenMu.Lock()
	o.qlen = 0.2*o.qlen + 0.8*float32(cur_qlen)
	o.qlenMu.Unlock()
	return
}

func (o *FederatedOffloader) CheckAndEnq(req *http.Request) (*list.Element, bool) {
	log.Println("[DEBUG] in checkAndEnq")
	appname := extractEntityName(req)

	var historic_qlen float32
	o.qlenMu.RLock()
	historic_qlen = o.qlen
	o.qlenMu.RUnlock()

	var instantaneous_qlen float32
	instantaneous_qlen = o.Finfo.getSnapshot().Qlen

	log.Printf("[DEBUG ] %s Instantaneous qlen: %f, historic qlen: %f\n", appname, instantaneous_qlen, historic_qlen)
	o.Finfo.mu.Lock()
	defer o.Finfo.mu.Unlock()

	isOffloaded := o.IsOffloaded(req)
	cur_qlen := instantaneous_qlen
	if isOffloaded {
		cur_qlen = historic_qlen
	}

	log.Println("[DEBUG ] qlen: ", cur_qlen)
	log.Println("[DEBUG] qlen_max", int(o.Qlen_max))
	go o.update_qlen()
	if int(cur_qlen) < int(o.Qlen_max) {
		log.Println("[DEBUG] inside if branch", int(o.Qlen_max))
		return o.Finfo.invoke_list.PushBack(time.Now()), true
	} else {
		return nil, false
	}
}

func (o *FederatedOffloader) stateUpdateRoutine() {
	defer o.wg.Done()

	qlen_timer := time.NewTicker(time.Duration(100) * time.Millisecond)
	for {
		select {
		case <-o.quit:
			return
		case <-qlen_timer.C:
			o.wg.Add(1)
			go o.update_qlen()
		}
	}
}

func (o *FederatedOffloader) GetOffloadCandidate(req *http.Request) string {
	var candidate string
	candidate = o.Host

	wts := []weightedrand.Choice{}
	o.mapMu.Lock()
	for node, node_qlen := range o.qlenMap {
		if node == o.Host {
			continue
		}
		if node_qlen > 10 {
			continue
		}
		// the randomized pickers expects nonnegative ints. So bound it to [0,1000]
		wt := 1000 - uint(node_qlen*100)
		wts = append(wts, weightedrand.NewChoice(node, wt))
	}
	o.mapMu.Unlock()
	chooser, err := weightedrand.NewChooser(wts...)
	if err != nil {
		log.Println("Unable to select chooser: ", err)
		return candidate
	}
	candidate = chooser.Pick().(string)
	return candidate
}

func (o *FederatedOffloader) Close() {
	close(o.quit)
	o.wg.Wait()
}

func (o *FederatedOffloader) GetStatusStr() string {
	log.Println("[DEBUG] in federated getStatusStr")
	snap := Snapshot{}
	o.qlenMu.RLock()
	snap.Qlen = o.qlen
	o.qlenMu.RUnlock()
	snap.HasCapacity = (snap.Qlen <= float32(o.Qlen_max))
	jbytes, err := json.Marshal(snap)
	if err != nil {
		log.Println("[WARNING] could not marshall status: ", err)
	}
	return string(jbytes)

}

func (o *FederatedOffloader) PostOffloadUpdate(snap Snapshot, target string) {
	o.mapMu.Lock()
	defer o.mapMu.Unlock()
	o.qlenMap[target] = snap.Qlen
}

func (o *FederatedOffloader) MetricSMAnalyze(ctx *list.Element) {

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
