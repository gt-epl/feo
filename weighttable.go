package main

import (
)

// The fact that there are separate latency-based offloaders doing different things makes this design unncessarily complicated.
type WeightTable struct {
	Finfo      FunctionInfo
	RouterList []router
}
