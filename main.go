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
	"net/http/httputil"
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
	host string
	// offloader 		OffloaderIntf
	config         FeoConfig
	policy         OffloadPolicy
	dagMap         map[string]*FaasEdgeDag
	applicationMap map[string]*Application
}

var local, offload atomic.Int32

var client = http.Client{
	Timeout: 20 * time.Second,
}

func (o *requestHandler) createProxyReq(originalReq *http.Request, target string, isOffload bool, port string) *http.Request {
	ODMN_PORT := "9696"
	// FAAS_PORT := "3233"
	FAAS_PORT := port

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
	newBody := bytes.NewBuffer(bodyBytes)
	originalReq.Body.Close() //  must close
	originalReq.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	upstreamReq, err := http.NewRequest(originalReq.Method, url.String(), newBody)
	upstreamReq.Header = originalReq.Header
	upstreamReq.Header.Set("Content-Length", strconv.Itoa(len(bodyBytes)))
	upstreamReq.Header.Add("Transfer-Encoding", "identity")
	upstreamReq.TransferEncoding = []string{"identity"}
	upstreamReq.ContentLength = originalReq.ContentLength

	if isOffload {
		upstreamReq.Header.Set("X-Offloaded-For", o.host)
	}

	if err != nil {
		//should not happen
		return nil
	}

	return upstreamReq
}

func (r *requestHandler) handleRegisterActionRequest(w http.ResponseWriter, req *http.Request) {
	// curl -X PUT 'http://localhost:9696/api/v1/namespaces/guest/actions/test?initPort=9000&numReplicas=10'
	appName := strings.Split(req.URL.Path, "/")[6]
	log.Printf("Handle register request for %s", appName)
	if appName == "" {
		http.Error(w, fmt.Sprintf("appName not present in URL Path: %q", req.URL.Path), http.StatusBadRequest)
	}

	queryParams := req.URL.Query()
	initPortNumberStr := queryParams.Get("initPort")
	initPortNumber, err := strconv.Atoi(initPortNumberStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("initPort %q is not a valid integer", initPortNumberStr), http.StatusBadRequest)
	}

	numReplicasStr := queryParams.Get("numReplicas")
	numReplicas, err := strconv.Atoi(numReplicasStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("numReplicas %q is not a valid integer", numReplicasStr), http.StatusBadRequest)
	}

	var offloader OffloaderIntf
	if appName == "fiblocal2" {
		offloader = OffloadFactory("base", r.config)
	} else {
		offloader = OffloadFactory(r.policy, r.config)
	}
	offloader.SetMaxQlen(int32(numReplicas) * 2)

	// Note, calling this request multiple times for the same appName will result in a completely new offloader & portChan created.
	app := createApplication(appName, initPortNumber, numReplicas, offloader)
	r.applicationMap[appName] = app
	log.Printf("Created Or Updated application %s", appName)
}

func (r *requestHandler) handleInvokeActionRequest(w http.ResponseWriter, req *http.Request) {

	var resp *http.Response
	var ctx *list.Element
	appName := strings.Split(req.URL.Path, "/")[6]
	log.Println("Recv req for applicaton", appName)
	app, ok := r.applicationMap[appName]
	if !ok {
		http.Error(w, fmt.Sprintf("Application %s does not exist", appName), http.StatusNotFound)
		log.Fatalf("Application %s does not exist", appName)
	}

	if app == nil {
		log.Fatalf("app is nil for app %s", appName)
	}

	if app.offloader == nil {
		log.Fatalf("Offloader is null for app %s", appName)

		if appName == "fiblocal2" {
			r.applicationMap[appName].offloader = OffloadFactory("base", r.config)
		} else {
			r.applicationMap[appName].offloader = OffloadFactory(r.policy, r.config)
		}
	}

	offloader := app.offloader

	metricCtx := offloader.MetricSMInit()

	log.Println("Recv req for applicaton", appName)
	ctx, localExecution := offloader.CheckAndEnq(req)
	snap := offloader.GetSnapshot(req)
	w.Header().Set("InstQLEN", strconv.FormatInt(int64(snap.Qlen), 10))
	w.Header().Set("HistQLEN", strconv.FormatFloat(float64(snap.HistoricQlen), 'E', -1, 32))

	if offloader.IsOffloaded(req) {
		//set offloadee header details
		w.Header().Set("Content-Type", "application/json")
		stat := offloader.GetStatusStr()
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
		if offloader.IsOffloaded(req) {
			return
		}

		// Begin OFFLOAD Steps
		for retry_count := 0; retry_count < RETRY_MAX; retry_count++ {

			// When do we get out of the loop?
			// Should we break on a successful offload?
			offloader.MetricSMAdvance(metricCtx, MetricSMState("PREOFFLOADSEARCH"))
			candidate := offloader.GetOffloadCandidate(req)
			offloader.MetricSMAdvance(metricCtx, MetricSMState("OFFLOADSEARCH"))

			getCandidateTime := metricCtx.Value.(*MetricSM).offloadSearch.Sub(metricCtx.Value.(*MetricSM).preOffloadSearch).String()
			w.Header().Set("Get-Candidate", getCandidateTime)

			if candidate == r.host {
				localExecution = true
				break
			}

			localExecution = false
			log.Println("[INFO] offload to ", candidate)
			proxyReq := r.createProxyReq(req, candidate, true, "0" /*Doesn't matter in the case of offload*/)
			var err error
			offloader.MetricSMAdvance(metricCtx, MetricSMState("PREOFFLOAD"), candidate)
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
					offloader.MetricSMAdvance(metricCtx, MetricSMState("POSTOFFLOAD"))
					break
				}
				log.Println("[DEBUG] failed offload execution")
				w.Header().Set("OffloadReject", "true")
				offloader.PostOffloadUpdate(snap, candidate)

				// w.Header().Set("Offload-Reject", "1")
			}
		}

		// Cannot find a node to offload. Execute locally
		if localExecution {
			ctx = offloader.ForceEnq(req)
		}
	}

	// NOTE: this is not as an "else" block because local execution is possible despite taking the first branch
	var port string
	if localExecution {
		log.Println("Forwarding to local FaaS Node")
		// self local processing
		// conditions: localEnq was successful OR offload failed and forced enq

		// Since in a FaaS Platform, choosing the candidate container would add to the POSTLOCAL-PRELOCAL latency.
		offloader.MetricSMAdvance(metricCtx, MetricSMState("PRELOCAL"), r.host)

		// select {

		// case msg := <-app.portChan:
		// 	port = msg
		// }
		port = <-app.portChan

		contentLength := req.Header.Get("Content-Length")
		proxyReq := r.createProxyReq(req, r.host, false, port)
		proxyReq.Header.Add("Content-Length", contentLength)
		proxyReq.Header.Add("Transfer-Encoding", "identity")
		proxyReq.TransferEncoding = []string{"identity"}
		proxyReq.ContentLength = req.ContentLength

		var err error
		resp, err = client.Do(proxyReq)

		if err != nil {
			//something bad happened
			log.Println("%s: local processing returned error %s", appName, err.Error())
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		} else if resp.StatusCode != http.StatusOK {
			respmsg, _ := io.ReadAll(resp.Body)
			log.Println(fmt.Sprintf("%s: Bad http response: %s", appName, string(respmsg)), resp.StatusCode)
			http.Error(w, fmt.Sprintf("Bad http response: %s", string(respmsg)), resp.StatusCode)
			dump, _ := httputil.DumpRequestOut(proxyReq, true)
			log.Println(dump)
			panic(nil)
			return
		} else {
			// This is in the critical path.
			offloader.MetricSMAdvance(metricCtx, MetricSMState("POSTLOCAL"))
		}

		offloader.Deq(req, ctx)
		w.Header().Set("Invoc-Loc", "Local")
		local.Add(1)
	} else {
		w.Header().Set("Invoc-Loc", "Offload")
		offload.Add(1)
	}

	log.Printf("Local,Offload=%d,%d\n", local.Load(), offload.Load())

	offloader.MetricSMAdvance(metricCtx, MetricSMState("FINAL"))

	offloader.MetricSMAnalyze(metricCtx)

	timeElapsedInExec := offloader.MetricSMElapsed(metricCtx)
	if timeElapsedInExec != "" {
		w.Header().Set("Invoc-Time", timeElapsedInExec)
	}

	io.Copy(w, resp.Body)
	if resp.Body != nil {
		resp.Body.Close()
	} else {
		log.Println("Response is empty!")
	}

	if localExecution {
		app.portChan <- port
	}

	offloader.MetricSMDelete(metricCtx)
	log.Println("Successfully handle request")
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
	traverseResultBytes, err := d.TraverseDag(rootId, w, req)
	if err != nil {
		log.Printf("Traversing dag %s failed: %s", dagName, err.Error())
		http.Error(w, fmt.Sprintf("error while traversing the DAG %s: %s", dagName, err.Error()), http.StatusBadRequest)
		return
	}

	// Marshal the results and write reply
	//log.Printf("Final resp is %s", string(traverseResultBytes))
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
		switch req.Method {
		case "PUT":
			r.handleRegisterActionRequest(w, req)
		case "POST":
			r.handleInvokeActionRequest(w, req)
		}
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
		Handler:        &requestHandler{policy: OffloadPolicy(config.Policy.Name), config: config, applicationMap: map[string]*Application{}, host: config.Host, dagMap: map[string]*FaasEdgeDag{}},
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(s.ListenAndServe())
	// cur_offloader.Close()
	for _, app := range (s.Handler).(*requestHandler).applicationMap {
		app.offloader.Close()
	}
}
