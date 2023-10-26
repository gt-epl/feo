// Based on documentation at https://pkg.go.dev/net/http#ListenAndServe
package main

import (
	// "bytes"
	// "fmt"
	"log"
	"io"
	// "io/ioutil"
	// "encoding/json"
	"os"
	"strings"
	"sync"
	"time"
	"net/http"
	"net/url"
)

type router struct {
	scheme string
	host string
	weight int
	// Microseconds
	timeElapsed float64
}
var routerList []router

type routerHandler struct{}
var client http.Client

var backendUrlScheme string
var backendUrlHost string

var reqCount int
var routerListMutex sync.RWMutex
var routerMutex []sync.RWMutex


func PopulateForwardList(routerListString string, myHostAddr string) {
	routerStringList := strings.Split(routerListString, "\n")
	for _,routerString := range routerStringList {
		routerUrl, err := url.Parse(routerString)
		if (err!=nil) {
			log.Fatal("PopulateForwardList: Error parsing router list")
		}
		if (routerUrl.Host != myHostAddr) {
			var newRouter router
			newRouter.scheme = routerUrl.Scheme
			newRouter.host = routerUrl.Host
			newRouter.weight = 1
			newRouter.timeElapsed = 0.0
			routerList = append(routerList, newRouter)
			
			// var newRouterMutex sync.RWMutex
			routerMutex = append(routerMutex, sync.RWMutex{})
		}
	}
}

func GetRouterIdx() int {
	// We need to decide which router to forward the request to
	return 0
}

func UpdateRouterWeight(elapsed time.Duration, idx int) {
	// Updating the weight of the peer router. We don't have to do this always. But when we do, we need to take a lock.
	alpha := 0.1
	routerMutex[idx].Lock()
	routerList[idx].timeElapsed = routerList[idx].timeElapsed * (1-alpha) + float64(elapsed.Microseconds()) * alpha
	log.Println(routerList[idx].timeElapsed)
	routerMutex[idx].Unlock()
}

// Should we create the client outside the handler? But there can be simulate
func (*routerHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	// requestBody, _ := json.Marshal(map[string]string{
	// 	"input" : "hello",
	// })

	// req1, err:= http.NewRequest("POST", "http://127.0.0.1:3233/api/v1/namespaces/guest/actions/copy?blocking=true&result=true", bytes.NewBuffer(requestBody))
	// if (err != nil) {
	// 	fmt.Fprintf(w, "Fail\n");
	// }

	// req1.Header.Add("Authorization", "Basic MjNiYzQ2YjEtNzFmNi00ZWQ1LThjNTQtODE2YWE0ZjhjNTAyOjEyM3pPM3haQ0xyTU42djJCS0sxZFhZRnBYbFBrY2NPRnFtMTJDZEFzTWdSVTRWck5aOWx5R1ZDR3VNREdJd1A=")
	// req1.Header.Set("Content-Type", "application/json")
	// req1.Header.Set("User-Agent", "OpenWhisk-CLI/1.0 linux amd64")

	// We could use time as the factor to decide whether to offload?
	
	// NOTE: We need to change this to read mutexes when we are not using these count based routing.
	var reqCountLocal int
	routerListMutex.Lock()
	reqCount += 1
	reqCountLocal = reqCount
	routerListMutex.Unlock()
	
	forwardedField := req.Header.Get("X-Forwarded-For")
	routerIdx := -1
	// NOTE: Only if this has not been forwarded by another router. DESIGN DECISION!
	if (len(strings.TrimSpace(forwardedField)) == 0 && reqCountLocal%5 == 0) {
		routerIdx = GetRouterIdx()
		req.URL.Scheme = routerList[routerIdx].scheme
		req.URL.Host = routerList[routerIdx].host
		req.RequestURI = ""
		req.Header.Set("X-Forwarded-For", req.RemoteAddr)
		// log.Println("Forwarding to another router")
	} else {
		req.URL.Scheme = backendUrlScheme
		req.URL.Host = backendUrlHost
		req.RequestURI = ""
		// Use this to figure out if forwared or not?
		req.Header.Set("X-Forwarded-For", req.RemoteAddr)
	}

	var now time.Time
	if (routerIdx != -1) {
		now = time.Now()
	}
	
	resp, err := client.Do(req)
	if (err != nil) {
		http.Error(w, err.Error(), http.StatusBadGateway)
		// resp.Body.Close()?
	} else {
		// body,_ := ioutil.ReadAll(resp.Body)
		// fmt.Fprintf(w, string(body))

		if (routerIdx != -1) {
			elapsed := time.Since(now)
			UpdateRouterWeight(elapsed, routerIdx)
		}

		io.Copy(w, resp.Body)
		resp.Body.Close()
	}
	
}

func main() {
	// client := &http.Client{}

	backendUrl,err := url.Parse(os.Args[2])
	if (err != nil) {
		log.Fatal("Error in backend url provided")
	}
	backendUrlScheme = backendUrl.Scheme
	if (backendUrlScheme == "") {
		log.Fatal("Error in backend url provided")
	}
	backendUrlHost = backendUrl.Host
	if (backendUrlHost == "") {
		log.Fatal("Error in backend url provided")
	}

	routerListString, err := os.ReadFile("routerlist.txt")
	if (err != nil) {
		log.Fatal("Error in reading router list file.")
	}

	// NOTE: We need os.Args[1] to be the value that we are going to use in the router list file!!
	PopulateForwardList(string(routerListString), os.Args[1])
	for _,ele := range routerList {
		log.Println(ele.host)
	}

	//
	reqCount = 0
	s := &http.Server{
		Addr:           ":" + strings.Split(os.Args[1], ":")[1],
		Handler:        &routerHandler{},
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(s.ListenAndServe())
}