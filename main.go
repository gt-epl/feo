// Based on documentation at https://pkg.go.dev/net/http#ListenAndServe
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"

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
	host      string //local faas node
	offloader OffloaderIntf
}

var client http.Client

var backendUrlScheme string
var backendUrlHost string

// TODO: use a yaml/json encoder to convert this.
func PopulateForwardList(routerListString string, myHostAddr string) []router {

	var routerList []router
	routerStringList := strings.Split(routerListString, "\n")
	for _, routerString := range routerStringList {
		routerUrl, err := url.Parse(routerString)
		if err != nil {
			log.Fatal("PopulateForwardList: Error parsing router list")
		}
		if routerUrl.Host != myHostAddr {
			var newRouter router
			newRouter.scheme = routerUrl.Scheme
			newRouter.host = routerUrl.Host
			newRouter.weight = 1
			newRouter.timeElapsed = 0.0
			routerList = append(routerList, newRouter)
		}
	}
	return routerList
}

func createProxyReq(originalReq *http.Request, offloadee string) *http.Request {
	return originalReq
}

func (r *offloadHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	var resp *http.Response
	defer resp.Body.Close()
	localExecution := r.offloader.checkAndEnq(req)

	if !localExecution {
		if r.offloader.isOffloaded(req) {
			// return Neg Ack from offloadee.
			w.Header().Set("Content-Type", "application/json")

			//set offloadee header details
			w.Header().Set("Offload-Status", r.offloader.getStatusStr())
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "")
		}

		// Begin OFFLOAD Steps
		candidate := r.offloader.getOffloadCandidate(req)
		if candidate != r.host {
			proxyReq := createProxyReq(req, candidate)
			var err error
			resp, err = client.Do(proxyReq)
			if err != nil {
				log.Printf("[WARN] offload http request failed")
				localExecution = true
			} else {
				jstr := resp.Header.Get("Offload-Status")
				snap := Snapshot{}
				err := json.Unmarshal([]byte(jstr), &snap)
				if err != nil {
					localExecution = !snap.HasCapacity
				}
			}
		} else {
			localExecution = true
		}

		if localExecution {
			r.offloader.forceEnq(req)
		}
	}

	// NOTE: this is not as an "else" block because local execution is possible despite taking the first branch
	if localExecution {
		// self local processing
		// conditions: localEnq was successful OR offload failed and forced enq
		var err error
		proxyReq := createProxyReq(req, "self")
		resp, err = client.Do(proxyReq)
		if err != nil {
			//something bad happened
			log.Println("local processing returned error", err)
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
	}
	io.Copy(w, resp.Body)
}

func main() {
	// client := &http.Client{}

	backendUrl, err := url.Parse(os.Args[2])
	if err != nil {
		log.Fatal("Error in backend url provided")
	}
	backendUrlScheme = backendUrl.Scheme
	if backendUrlScheme == "" {
		log.Fatal("Error in backend url provided")
	}
	backendUrlHost = backendUrl.Host
	if backendUrlHost == "" {
		log.Fatal("Error in backend url provided")
	}

	routerListString, err := os.ReadFile("routerlist.txt")
	if err != nil {
		log.Fatal("Error in reading router list file.")
	}

	// NOTE: We need os.Args[1] to be the value that we are going to use in the router list file!!
	routerList := PopulateForwardList(string(routerListString), os.Args[1])
	for _, ele := range routerList {
		log.Println(ele.host)
	}

	//TODO: use gflags instead of os.Args
	policy := OffloadPolicy("alternate")
	cur_offloader := OffloadFactory(policy, routerList, os.Args[1])

	s := &http.Server{
		Addr:           ":" + strings.Split(os.Args[1], ":")[1],
		Handler:        &offloadHandler{offloader: cur_offloader},
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(s.ListenAndServe())
}
