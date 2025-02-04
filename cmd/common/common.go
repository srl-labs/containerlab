package common

import (
	"time"
)

var (
	Debug   bool
	Timeout time.Duration
)

// path to the topology file.
var Topo string

var (
	VarsFile string
	Graph    bool
	Runtime  string
)

// subset of nodes to work with.
var NodeFilter []string

// lab Name.
var Name string

// Use graceful shutdown.
var Graceful bool
