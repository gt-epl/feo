package main

import (
	"net/http"
)

type AlternateOffloader struct {
	*BaseOffloader //embedding
	local_flag     bool
}

func (o *AlternateOffloader) checkAndEnq(req *http.Request) bool {
	return true
}

func (o *AlternateOffloader) getOffloadCandidate(req *http.Request) string {
	var candidate string
	if o.local_flag {
		candidate = o.host
	} else {
		candidate = o.routerList[1].host
	}
	o.local_flag = !o.local_flag
	return candidate
}
