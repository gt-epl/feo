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

type QListInfo struct {
	ts  time.Time
	len int
}

type FederatedOffloader struct {
	*BaseOffloader
	cur_idx int

	qlenMap map[string]float32
	mapMu   sync.Mutex
	// wg              sync.WaitGroup
	// quit            chan bool
	// qlens_over_time []QListInfo
}

func NewFederatedOffloader(base *BaseOffloader) *FederatedOffloader {
	fed := &FederatedOffloader{cur_idx: 0, BaseOffloader: base}
	fed.Qlen_max = 10
	log.Println("[DEBUG] qlen_max = ", fed.Qlen_max)
	fed.qlenMap = make(map[string]float32)
	for _, r := range fed.RouterList {
		fed.qlenMap[r.host] = 0
	}

	// fed.wg.Add(1)
	// go fed.stateUpdateRoutine()
	return fed
}

func (o *FederatedOffloader) CheckAndEnq(req *http.Request) (*list.Element, bool) {
	log.Println("[DEBUG] in checkAndEnq")

	o.Finfo.mu.Lock()
	defer o.Finfo.mu.Unlock()

	instantaneous_qlen := o.Finfo.invoke_list.Len()
	historic_qlen := o.Finfo.historic_qlen

	// appname := extractEntityName(req)
	// log.Printf("[DEBUG ] %s Instantaneous qlen: %f, historic qlen: %f\n", appname, instantaneous_qlen, historic_qlen)

	cur_time := time.Now()

	o.qlen_info_chan <- QListInfo{cur_time, instantaneous_qlen + 1}
	if historic_qlen < float32(o.Qlen_max) {
		return o.Finfo.invoke_list.PushBack(cur_time), true
	} else {
		return nil, false
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
	// snap := Snapshot{}
	snap := o.Finfo.getSnapshot()
	// o.qlenMu.RLock()
	// snap.Qlen = o.qlen
	// o.qlenMu.RUnlock()
	snap.HasCapacity = (snap.Qlen <= int(o.Qlen_max))
	jbytes, err := json.Marshal(snap)
	if err != nil {
		log.Println("[WARNING] could not marshall status: ", err)
	}
	return string(jbytes)

}

func (o *FederatedOffloader) PostOffloadUpdate(snap Snapshot, target string) {
	o.mapMu.Lock()
	defer o.mapMu.Unlock()
	o.qlenMap[target] = float32(snap.Qlen)
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
