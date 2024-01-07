package main

import (
	"log"

	"github.com/heimdalr/dag"
)

type DagVertex struct {
	StageName          string   `yaml:"stage_name"`
	ActionName         string   `yaml:"action_name"`
	DownstreamVertices []string `yaml:"downstream_vertices"`
	ShouldNotOffload   bool     `yaml:"should_not_offload"`
}

type DagManifest struct {
	Name     string       `yaml:"name"`
	Vertices []*DagVertex `yaml:"vertices"`
}

type FaasEdgeDag struct {
	Name string
	dag  *dag.DAG
}

func (d *DagVertex) ID() string {
	return d.StageName
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
