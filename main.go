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

// TODO: This assumes that feo's endpoint is fixed. A better parser/http framework is needed to avoid this hardcode
func extractEntityName(req *http.Request) string {
	return strings.Split(req.URL.Path, "/")[6]
}

type requestHandler struct {
	host      		string
	// offloader 		OffloaderIntf
	config 			FeoConfig
	policy			OffloadPolicy
	offloaderMap	map[string]OffloaderIntf
	dagMap    		map[string]*FaasEdgeDag
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

	var resp *http.Response
	var ctx *list.Element
	appName := strings.Split(req.URL.Path, "/")[6]
	_, ok := r.offloaderMap[appName]

	if !ok {
		r.offloaderMap[appName] = OffloadFactory(r.policy, r.config)
	}

	metricCtx := r.offloaderMap[appName].MetricSMInit()

	log.Println("Recv req for applicaton", appName)
	ctx, localExecution := r.offloaderMap[appName].CheckAndEnq(req)

	if r.offloaderMap[appName].IsOffloaded(req) {
		//set offloadee header details
		w.Header().Set("Content-Type", "application/json")
		stat := r.offloaderMap[appName].GetStatusStr()
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
		if r.offloaderMap[appName].IsOffloaded(req) {
			return
		}

		// Begin OFFLOAD Steps
		for retry_count := 0; retry_count < RETRY_MAX; retry_count++ {

			// When do we get out of the loop?
			// Should we break on a successful offload?

			candidate := r.offloaderMap[appName].GetOffloadCandidate(req)
			r.offloaderMap[appName].MetricSMAdvance(metricCtx, MetricSMState("OFFLOADSEARCH"))
			if candidate == r.host {
				localExecution = true
				break
			}

			localExecution = false
			log.Println("[INFO] offload to ", candidate)
			proxyReq := r.createProxyReq(req, candidate, true)
			var err error
			r.offloaderMap[appName].MetricSMAdvance(metricCtx, MetricSMState("PREOFFLOAD"), candidate)
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
					r.offloaderMap[appName].MetricSMAdvance(metricCtx, MetricSMState("POSTOFFLOAD"))
					break
				}
				log.Println("[DEBUG] failed offload execution")
				r.offloaderMap[appName].PostOffloadUpdate(snap, candidate)
			}
		}

		// Cannot find a node to offload. Execute locally
		if localExecution {
			ctx = r.offloaderMap[appName].ForceEnq(req)
		}
	}

	// NOTE: this is not as an "else" block because local execution is possible despite taking the first branch
	if localExecution {
		log.Println("Forwarding to local FaaS Node")
		// self local processing
		// conditions: localEnq was successful OR offload failed and forced enq
		var err error
		proxyReq := r.createProxyReq(req, r.host, false)
		r.offloaderMap[appName].MetricSMAdvance(metricCtx, MetricSMState("PRELOCAL"), r.host)
		resp, err = client.Do(proxyReq)
		if err != nil {
			//something bad happened
			log.Println("local processing returned error", err)
			http.Error(w, err.Error(), http.StatusBadGateway)
		} else if resp.StatusCode != http.StatusOK {
			log.Println("Bad http response", resp.StatusCode)
		} else {
			// This is in the critical path.
			r.offloaderMap[appName].MetricSMAdvance(metricCtx, MetricSMState("POSTLOCAL"))
		}

		r.offloaderMap[appName].Deq(req, ctx)
		w.Header().Set("Invoc-Loc", "Local")
		local.Add(1)
	} else {
		w.Header().Set("Invoc-Loc", "Offload")
		offload.Add(1)
	}

	log.Printf("Local,Offload=%d,%d\n", local.Load(), offload.Load())

	r.offloaderMap[appName].MetricSMAdvance(metricCtx, MetricSMState("FINAL"))

	r.offloaderMap[appName].MetricSMAnalyze(metricCtx)

	timeElapsedInExec := r.offloaderMap[appName].MetricSMElapsed(metricCtx)
	if (timeElapsedInExec != "") {
		w.Header().Set("Invoc-Time", timeElapsedInExec)
	}

	io.Copy(w, resp.Body)
	if resp.Body != nil {
		resp.Body.Close()
	} else {
		log.Println("Response is empty!")
	}

	r.offloaderMap[appName].MetricSMDelete(metricCtx)
	return
}

func (r *requestHandler) handleUploadDagRequest(w http.ResponseWriter, req *http.Request) {
	// Read the binary data from the request body
	binaryData, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	// curl -X POST -H "Content-Type: application/x-yaml" --data-binary "@apps/dag/dag_manifest.yml" http://localhost:9696/api/v1/namespaces/guest/dag/test
	var newDagManifest DagManifest
	if err := yaml.Unmarshal(binaryData, &newDagManifest); err != nil {
		log.Fatal(err)
	}

	// Process the YAML data as needed
	log.Printf("Received YAML data: %+v\n", newDagManifest)

	dag := createDag(newDagManifest)
	r.dagMap[dag.Name] = dag
}

func (r *requestHandler) handleInvokeDagRequest(w http.ResponseWriter, req *http.Request) {
	// Lookup the DAG from in-memory storage
	dagName := extractEntityName(req)
	log.Println("InvokeDAG with name", dagName)
	d, ok := r.dagMap[dagName]
	if !ok {
		http.Error(w, fmt.Sprintf("DAG with name %s does not exist", dagName), http.StatusNotFound)
		return
	}

	// Find the root vertex
	rootList := d.dag.GetRoots()
	if len(rootList) != 1 {
		http.Error(w, fmt.Sprintf("The DAG %s has %d roots. For now, we only assume 1 root.", dagName, len(rootList)), http.StatusBadRequest)
		return
	}
	var rootId string
	for id := range rootList {
		rootId = id
	}

	// Traverse the DAG starting from the root vertex
	traverseResultBytes, err := d.TraverseDag(rootId, req)
	if err != nil {
		log.Printf("Traversing dag %s failed: %s", dagName, err.Error())
		http.Error(w, fmt.Sprintf("error while traversing the DAG %s: %s", dagName, err.Error()), http.StatusBadRequest)
		return
	}

	// Marshal the results and write reply
	log.Printf("Final resp is %s", string(traverseResultBytes))
	respBody := io.NopCloser(strings.NewReader(string(traverseResultBytes)))
	io.Copy(w, respBody)
	if respBody != nil {
		respBody.Close()
	} else {
		log.Println("Response is empty!")
	}
}

func (r *requestHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log.Println("Receive request!")
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
	log.SetFlags(log.LstdFlags | log.Lshortfile)
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
	// policy := OffloadPolicy(config.Policy.Name)
	// cur_offloader := OffloadFactory(policy, config)

	s := &http.Server{
		Addr:           config.Host,
		Handler:        &requestHandler{policy: OffloadPolicy(config.Policy.Name), config: config, offloaderMap: map[string]OffloaderIntf{}, host: config.Host, dagMap: map[string]*FaasEdgeDag{}},
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(s.ListenAndServe())
	// cur_offloader.Close()
	for _, offloader := range (s.Handler).(*requestHandler).offloaderMap {
		offloader.Close()
	}
}
