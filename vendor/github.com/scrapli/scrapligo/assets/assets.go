package assets

import "embed"

// Assets is the embedded assets objects for the included platform yaml data.
//
//go:embed platforms/*
var Assets embed.FS
