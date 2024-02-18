package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/heimdalr/dag"
)

type DagVertex struct {
	StageName          string   `yaml:"stage_name"`
	ActionName         string   `yaml:"action_name"`
	DownstreamVertices []string `yaml:"downstream_vertices"`
	ShouldNotOffload   bool     `yaml:"should_not_offload"`
	ConditionalKey     string   `yaml:"conditional_key"`
	ConditionalValue   bool     `yaml:"conditional_value"`
}

type DagManifest struct {
	Name     string       `yaml:"name"`
	Vertices []*DagVertex `yaml:"vertices"`
}

type FaasEdgeDag struct {
	Name string
	dag  *dag.DAG
}

// This function takes the result of multiple functions, and appends them into 1 big payload object.
// Each element in the input holds a byte array, which encodes a JSON object (payload).
// The output is one big byte array, or one big JSON object (payload) encoded.
// The function replaces the bordernig parenthesis ('}{') with a comma character (',') to append multiple items.
//
// e.g.)
// Input: [`{"name": "John Doe"}`, `{"age": 42}`]    (type [][]byte)
// Output: `{"name": "John Doe", "age": 42}`         (type []byte)
func appendFlowResults(flowResults []dag.FlowResult) ([]byte, error) {
	result := []byte{}

	for idx, r := range flowResults {
		log.Printf("Decoding result of function  %s", r.ID)
		var itemBytes []byte
		itemBytes, ok := r.Result.([]byte)
		if !ok {
			return nil, fmt.Errorf("failed to assert []byte for parent %s\n", r.ID)
		}
		if idx == 0 {
			result = append(result, itemBytes...)
		} else {
			// 44 is the ASCII char for ','.
			result = append(result[:len(result)-1], byte(44))
			result = append(result, itemBytes[1:]...)
		}
	}

	return result, nil
}

func composeReqBody(vertexID, rootID string, req *http.Request, parentFlowResults []dag.FlowResult) ([]byte, int, error) {
	// Concatenate results of parent functions
	if vertexID == rootID {
		log.Printf("Using provided req body for root id\n")
		bodyBytes, _ := io.ReadAll(req.Body)
		return bodyBytes, len(bodyBytes), nil
	}

	// Create input for this function
	parentOutputsByte, err := appendFlowResults(parentFlowResults)
	if err != nil {
		return nil, 0, err
	}
	return parentOutputsByte, len(parentOutputsByte), nil
}

const CONDITIONAL_STOP = "CONDITIONAL_STOP"

func (d *FaasEdgeDag) TraverseDag(rootID string, w http.ResponseWriter, req *http.Request) ([]byte, error) {
	vertexCallback := func(d *dag.DAG, vertexID string, parentResults []dag.FlowResult) (interface{}, error) {
		for _, pr := range parentResults {
			if pr.Error != nil && strings.Contains(pr.Error.Error(), CONDITIONAL_STOP) {
				log.Printf("Parent %s saw a conditional stop \n", pr.ID)
				return pr.Result, pr.Error
			}
		}

		v, _ := d.GetVertex(vertexID)
		dv, ok := v.(*DagVertex)
		if !ok {
			return nil, fmt.Errorf("Error asserting DagVertex type for id %s\n", vertexID)
		}
		log.Printf("flowCallback called for vertex %s", vertexID)

		// Put together http request to loop back to feo for this one function
		client := &http.Client{}
		urlTemplate := "http://%s/api/v1/namespaces/guest/actions/%s?blocking=true&result=true"
		url := fmt.Sprintf(urlTemplate, req.Host, dv.ActionName)
		functionInputBytes, contentLength, err := composeReqBody(vertexID, rootID, req, parentResults)
		if err != nil {
			return nil, fmt.Errorf("failed to create req body for %s: %s", vertexID, err.Error())
		}
		functionInputBody := bytes.NewBuffer(functionInputBytes)
		invokeReq, err := http.NewRequest("POST", url, functionInputBody)
		if err != nil {
			return nil, fmt.Errorf("failed to create httprequest for %s: %s", vertexID, err.Error())
		}
		invokeReq.Header.Add("Authorization", "Basic MjNiYzQ2YjEtNzFmNi00ZWQ1LThjNTQtODE2YWE0ZjhjNTAyOjEyM3pPM3haQ0xyTU42djJCS0sxZFhZRnBYbFBrY2NPRnFtMTJDZEFzTWdSVTRWck5aOWx5R1ZDR3VNREdJd1A=")
		invokeReq.Header.Add("Content-Type", "application/json")
		invokeReq.Header.Add("Content-Length", strconv.Itoa(contentLength))
		invokeReq.TransferEncoding = []string{"identity"}
		invokeReq.Header.Add("Transfer-Encoding", "identity")

		// Launch request
		resp, err := client.Do(invokeReq)
		if err != nil {
			log.Printf("looped request for vertex %s returned error: %s", vertexID, err.Error())
			panic(nil)
			return nil, fmt.Errorf("looped request for vertex %s returned error: %s", vertexID, err.Error())
		} else if resp.StatusCode != http.StatusOK {
			log.Printf("bad http response for vertex %s %d", vertexID, resp.StatusCode)
			panic(nil)
			return nil, fmt.Errorf("bad http response for vertex %s %d", vertexID, resp.StatusCode)
		}

		// Unmarshal Response
		functionOutputBody, ok := resp.Body.(io.ReadCloser)
		if !ok {
			return nil, fmt.Errorf("failed to assert io.ReadCloser for resp.Body %+v\n", resp.Body)
		}
		functionOutput, err := io.ReadAll(functionOutputBody)
		if err != nil {
			return nil, fmt.Errorf("Cannot create io.Reader from functionOutput for %s", vertexID)
		}

		// Set execution time
		invocTime := resp.Header.Get("Invoc-Time")
		w.Header().Set(fmt.Sprintf("Invoc-Time-%s", vertexID), invocTime)
		invocLoc := resp.Header.Get("Invoc-Loc")
		w.Header().Set(fmt.Sprintf("Invoc-Loc-%s", vertexID), invocLoc)
		instQueueDepth := resp.Header.Get("InstQLEN")
		w.Header().Set(fmt.Sprintf("Invoc-Queue-Depth-%s", vertexID), instQueueDepth)
		histQueueDepth := resp.Header.Get("HistQLEN")
		w.Header().Set(fmt.Sprintf("Invoc-Historic-Depth-%s", vertexID), histQueueDepth)

		if dv.ConditionalKey != "" {
			var functionOutputJson map[string]interface{}
			var cond_test_value bool
			err := json.Unmarshal(functionOutput, &functionOutputJson)
			if err != nil {
				return nil, fmt.Errorf("Cannot unmarshal output for %s into empty interface: %s", vertexID, err.Error())
			}
			cond_test_value, ok = functionOutputJson[dv.ConditionalKey].(bool)
			if !ok || cond_test_value != dv.ConditionalValue {
				log.Printf("Conditional check skip triggered ok %t, cond_test_value %t\n", ok, cond_test_value)
				return functionOutput, fmt.Errorf(CONDITIONAL_STOP)
			}
		}

		return functionOutput, nil
	}

	traverseResults, err := d.dag.DescendantsFlow(rootID, nil, vertexCallback)
	if err != nil {
		return nil, err
	}

	for _, tr := range traverseResults {
		if tr.Error != nil && !strings.Contains(tr.Error.Error(), CONDITIONAL_STOP) {
			return nil, fmt.Errorf("Vertex %s saw error %s:", tr.ID, tr.Error.Error())
		}
	}

	finalResultPayload, err := appendFlowResults(traverseResults)
	if err != nil {
		return nil, fmt.Errorf("failed to apend flow result: %s", err.Error())
	}
	return finalResultPayload, nil
}

func (dv *DagVertex) ID() string {
	return dv.StageName
}

var _ dag.IDInterface = &DagVertex{}

func createDag(manifest DagManifest) *FaasEdgeDag {
	d := dag.NewDAG()

	// Action with same name
	for _, vertex := range manifest.Vertices {
		_, _ = d.AddVertex(vertex)
	}

	for _, vertex := range manifest.Vertices {
		for _, downstream_vertex_name := range vertex.DownstreamVertices {
			err := d.AddEdge(vertex.ID(), downstream_vertex_name)
			if err != nil {
				log.Printf("error adding edge, %s", err.Error())
			}
		}
	}

	log.Println(d.String())

	return &FaasEdgeDag{
		Name: manifest.Name,
		dag:  d,
	}
}
