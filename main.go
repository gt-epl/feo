// Based on documentation at https://pkg.go.dev/net/http#ListenAndServe
package main

import (
	"bytes"
	"container/list"
	"encoding/json"
	"fmt"
	"io"
	"log"

	"flag"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type router struct {
	scheme string
	host   string
	weight int
	// Microseconds
	timeElapsed float64
}

type offloadHandler struct {
	host      string
	offloader OffloaderIntf
}

var client http.Client

// TODO: use a yaml/json encoder to convert this.
func PopulateForwardList(routerListString string) []router {

	var routerList []router
	routerStringList := strings.Split(routerListString, "\n")
	for _, routerString := range routerStringList {
		if routerString == "" {
			continue
		}
		routerUrl, err := url.Parse(routerString)
		if err != nil {
			log.Fatal("PopulateForwardList: Error parsing router list")
		}

		var newRouter router
		newRouter.scheme = routerUrl.Scheme
		newRouter.host = routerUrl.Host
		newRouter.weight = 1
		newRouter.timeElapsed = 0.0
		routerList = append(routerList, newRouter)
	}
	return routerList
}

func (o *offloadHandler) createProxyReq(originalReq *http.Request, target string, isOffload bool) *http.Request {
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

	bodyBytes, err := io.ReadAll(originalReq.Body)
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

func (r *offloadHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	var resp *http.Response
	var ctx *list.Element
	log.Println("Recv req")
	ctx, localExecution := r.offloader.checkAndEnq(req)

	if !localExecution {
		if r.offloader.isOffloaded(req) {
			// return Neg Ack from offloadee.
			w.Header().Set("Content-Type", "application/json")

			//set offloadee header details
			log.Println("[DEBUG] send Neg ACK")
			stat := r.offloader.getStatusStr()
			log.Println("[DEBUG] status=", stat)
			w.Header().Set("Offload-Status", stat)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "")
			return
		}

		// Begin OFFLOAD Steps
		candidate := r.offloader.getOffloadCandidate(req)
		if candidate != r.host {
			log.Println("[INFO] offload to ", candidate)
			proxyReq := r.createProxyReq(req, candidate, true)
			var err error
			resp, err = client.Do(proxyReq)
			if err != nil {
				log.Println("[WARN] offload http request failed: ", err)
				localExecution = true
			} else {
				jstr := resp.Header.Get("Offload-Status")
				log.Println("[DEBUG] Successful Offload: ", resp.StatusCode, jstr)
				snap := Snapshot{}
				err := json.Unmarshal([]byte(jstr), &snap)
				if err == nil {
					localExecution = !snap.HasCapacity
				}
				resp.Body.Close()
			}
		} else {
			localExecution = true
		}

		if localExecution {
			ctx = r.offloader.forceEnq(req)
		}
	}

	// NOTE: this is not as an "else" block because local execution is possible despite taking the first branch
	if localExecution {
		log.Println("Forwarding to local FaaS Node")
		// self local processing
		// conditions: localEnq was successful OR offload failed and forced enq
		var err error
		proxyReq := r.createProxyReq(req, r.host, false)
		resp, err = client.Do(proxyReq)
		if err != nil {
			//something bad happened
			log.Println("local processing returned error", err)
			http.Error(w, err.Error(), http.StatusBadGateway)
		} else if resp.StatusCode != http.StatusOK {
			log.Println("Bad http response", resp.StatusCode)
		}

		r.offloader.Deq(req, ctx)
	}
	io.Copy(w, resp.Body)
	if resp.Body != nil {
		resp.Body.Close()
	} else {
		log.Println("Response is empty!")
	}
}

func main() {
	// client := &http.Client{}
	var nodelist = flag.String("nodelist", "routerlist.txt", "file containing line separated nodeip:port for offload candidates")
	var policystr = flag.String("policy", "", "offload policy")
	flag.Parse()

	//backendUrl := url.Parse("http://host:3233/api/v1/namespaces/guest/actions/copy?blocking=true&result=true")

	routerListString, err := os.ReadFile(*nodelist)
	if err != nil {
		log.Fatal("Error in reading router list file.")
	}

	// NOTE: We need os.Args[1] to be the value that we are going to use in the router list file!!
	routerList := PopulateForwardList(string(routerListString))
	for _, ele := range routerList {
		log.Println(ele.host)
	}

	//TODO: use gflags instead of os.Args
	policy := OffloadPolicy(*policystr)
	cur_offloader := OffloadFactory(policy, routerList, routerList[0].host)

	s := &http.Server{
		Addr:           routerList[0].host,
		Handler:        &offloadHandler{offloader: cur_offloader, host: routerList[0].host},
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(s.ListenAndServe())
}
