// Based on documentation at https://pkg.go.dev/net/http#ListenAndServe
package main

import (
	"bytes"
	"container/list"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strconv"
	"sync/atomic"

	"flag"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const RETRY_MAX = 1

type requestHandler struct {
	host      string
	offloader OffloaderIntf
}

var local, offload atomic.Int32

var client http.Client

func (o *requestHandler) createProxyReq(originalReq *http.Request, target string, isOffload bool) *http.Request {
	ODMN_PORT := "9696"
	FAAS_PORT := "3233"

	var newHost string
	ip := strings.Split(target, ":")[0]

	if isOffload {
		newHost = ip + ":" + ODMN_PORT
	} else {
		newHost = ip + ":" + FAAS_PORT
	}
	log.Println("[INFO] created Proxy newHost", newHost)

	url := url.URL{
		Scheme:   "http",
		Host:     newHost,
		Path:     originalReq.URL.Path,
		RawQuery: originalReq.URL.RawQuery,
	}

	//NOTE: why do this? Because you can only read the body once, therefore you read it completely and store it in a buffer and repopulate the req body. source: https://stackoverflow.com/questions/43021058/golang-read-request-body-multiple-times

	bodyBytes, _ := io.ReadAll(originalReq.Body)
	newBody := io.NopCloser(bytes.NewBuffer(bodyBytes))
	originalReq.Body.Close() //  must close
	originalReq.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	upstreamReq, err := http.NewRequest(originalReq.Method, url.String(), newBody)
	upstreamReq.Header = originalReq.Header

	if isOffload {
		upstreamReq.Header.Set("X-Offloaded-For", o.host)
	}

	if err != nil {
		//should not happen
		return nil
	}

	return upstreamReq
}

func (r *requestHandler) handleInvokeActionRequest(w http.ResponseWriter, req *http.Request) {

	metricCtx := r.offloader.MetricSMInit()
	var resp *http.Response
	var ctx *list.Element
	log.Println("Recv req")
	ctx, localExecution := r.offloader.CheckAndEnq(req)

	if r.offloader.IsOffloaded(req) {
		//set offloadee header details
		w.Header().Set("Content-Type", "application/json")
		stat := r.offloader.GetStatusStr()
		log.Println("[DEBUG] offload status ack=", stat)
		w.Header().Set(OffloadSuccess, strconv.FormatBool(localExecution))
		w.Header().Set(NodeStatus, stat)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "") //empty response
		// r.offloader.MetricSMAnalyze(metricCtx)
		// r.offloader.MetricSMDelete(metricCtx)
	}

	if !localExecution {

		//disallow nested offloads
		if r.offloader.IsOffloaded(req) {
			return
		}

		// Begin OFFLOAD Steps
		for retry_count := 0; retry_count < RETRY_MAX; retry_count++ {

			// When do we get out of the loop?
			// Should we break on a successful offload?

			candidate := r.offloader.GetOffloadCandidate(req)
			r.offloader.MetricSMAdvance(metricCtx, MetricSMState("OFFLOADSEARCH"))
			if candidate == r.host {
				localExecution = true
				break
			}

			localExecution = false
			log.Println("[INFO] offload to ", candidate)
			proxyReq := r.createProxyReq(req, candidate, true)
			var err error
			r.offloader.MetricSMAdvance(metricCtx, MetricSMState("PREOFFLOAD"), candidate)
			resp, err = client.Do(proxyReq)
			if err != nil {
				log.Println("[WARN] offload http request failed: ", err)
				localExecution = true
			} else {
				success, _ := strconv.ParseBool(resp.Header.Get(OffloadSuccess))
				jstr := resp.Header.Get(NodeStatus)
				log.Println("[DEBUG] Successful Offload Request: ", resp.StatusCode, jstr)
				snap := Snapshot{}
				err := json.Unmarshal([]byte(jstr), &snap)
				//only if offload-status present which is sent on neg-ack
				if err != nil {
					//never should happen
					log.Fatal("Failed parsing of node snapshot")
				}
				localExecution = !success
				if success {
					log.Println("[DEBUG] Successful offload execution")
					r.offloader.MetricSMAdvance(metricCtx, MetricSMState("POSTOFFLOAD"))
					break
				}
				log.Println("[DEBUG] failed offload execution")
				r.offloader.PostOffloadUpdate(snap, candidate)
			}
		}

		// Cannot find a node to offload. Execute locally
		if localExecution {
			ctx = r.offloader.ForceEnq(req)
		}
	}

	// NOTE: this is not as an "else" block because local execution is possible despite taking the first branch
	if localExecution {
		log.Println("Forwarding to local FaaS Node")
		// self local processing
		// conditions: localEnq was successful OR offload failed and forced enq
		var err error
		proxyReq := r.createProxyReq(req, r.host, false)
		r.offloader.MetricSMAdvance(metricCtx, MetricSMState("PRELOCAL"), r.host)
		resp, err = client.Do(proxyReq)
		if err != nil {
			//something bad happened
			log.Println("local processing returned error", err)
			http.Error(w, err.Error(), http.StatusBadGateway)
		} else if resp.StatusCode != http.StatusOK {
			log.Println("Bad http response", resp.StatusCode)
		} else {
			// This is in the critical path.
			r.offloader.MetricSMAdvance(metricCtx, MetricSMState("POSTLOCAL"))
		}

		r.offloader.Deq(req, ctx)
		local.Add(1)
	} else {
		offload.Add(1)
	}

	log.Printf("Local,Offload=%d,%d\n", local.Load(), offload.Load())

	io.Copy(w, resp.Body)
	if resp.Body != nil {
		resp.Body.Close()
	} else {
		log.Println("Response is empty!")
	}

	r.offloader.MetricSMAdvance(metricCtx, MetricSMState("FINAL"))

	r.offloader.MetricSMAnalyze(metricCtx)
	r.offloader.MetricSMDelete(metricCtx)
	return
}

func (r *requestHandler) handleUploadDagRequest(w http.ResponseWriter, req *http.Request) {
	log.Println("UploadDAG")
}

func (r *requestHandler) handleInvokeDagRequest(w http.ResponseWriter, req *http.Request) {
	log.Println("InvokeDAG")
}

func (r *requestHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Check the URL path and call the appropriate function based on the endpoint
	switch {
	case strings.HasPrefix(req.URL.Path, "/api/v1/namespaces/guest/actions/"):
		r.handleInvokeActionRequest(w, req)
	case strings.HasPrefix(req.URL.Path, "/api/v1/namespaces/guest/dag/"):
		switch req.Method {
		// Best practice would be to use 'POST' to upload/create a dag. However, for now, we use POST for invoking, to match with single action invoke.
		case "PUT":
			r.handleUploadDagRequest(w, req)
		case "POST":
			r.handleInvokeDagRequest(w, req)
		default:
			http.NotFound(w, req)
		}
	default:
		http.NotFound(w, req)
	}
}

func (r *requestHandler) handleUploadDagRequest(w http.ResponseWriter, req *http.Request) {
	log.Println("UploadDAG")
}

func (r *requestHandler) handleInvokeDagRequest(w http.ResponseWriter, req *http.Request) {
	log.Println("InvokeDAG")
}

func (r *requestHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Check the URL path and call the appropriate function based on the endpoint
	switch {
	case strings.HasPrefix(req.URL.Path, "/api/v1/namespaces/guest/actions/"):
		r.handleInvokeActionRequest(w, req)
	case strings.HasPrefix(req.URL.Path, "/api/v1/namespaces/guest/dag/"):
		switch req.Method {
		// Best practice would be to use 'POST' to upload/create a dag. However, for now, we use POST for invoking, to match with single action invoke.
		case "PUT":
			r.handleUploadDagRequest(w, req)
		case "POST":
			r.handleInvokeDagRequest(w, req)
		default:
			http.NotFound(w, req)
		}
	default:
		http.NotFound(w, req)
	}
}

func main() {
	// client := &http.Client{}
	var configstr = flag.String("config", "config.yml", "YML config for faas orchestrator")
	flag.Parse()

	f, err := os.ReadFile(*configstr)
	if err != nil {
		log.Fatal(err)
	}
	var config FeoConfig
	if err := yaml.Unmarshal(f, &config); err != nil {
		log.Fatal(err)
	}

	//telemetry
	local.Store(0)
	offload.Store(0)

	//backendUrl := url.Parse("http://host:3233/api/v1/namespaces/guest/actions/copy?blocking=true&result=true")

	for _, ele := range config.Peers {
		log.Println(ele)
	}

	//TODO: use gflags instead of os.Args
	policy := OffloadPolicy(config.Policy.Name)
	cur_offloader := OffloadFactory(policy, config)

	s := &http.Server{
		Addr:           config.Host,
		Handler:        &requestHandler{offloader: cur_offloader, host: config.Host},
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(s.ListenAndServe())
	cur_offloader.Close()
}
