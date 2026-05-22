package playground

import "embed"

//go:embed static/index.html static/assets/*
var staticFiles embed.FS
