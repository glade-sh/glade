package project

import (
	"bytes"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// flowShapeCache maps a Flow file's content hash to its parsed shape, so
// repeated identity checks read a file but parse it once. It is cleared when it
// grows past flowShapeCacheLimit.
var (
	flowShapeCacheMu sync.Mutex
	flowShapeCache   = map[[sha256.Size]byte]string{}
)

const flowShapeCacheLimit = 8192

func flowShapeForContent(data []byte) (string, bool) {
	key := sha256.Sum256(data)
	flowShapeCacheMu.Lock()
	shape, ok := flowShapeCache[key]
	flowShapeCacheMu.Unlock()
	if ok {
		return shape, true
	}
	var parsed flowRunModeShape
	if err := xml.Unmarshal(data, &parsed); err != nil {
		return "", false
	}
	shape = strings.TrimSpace(parsed.APIVersion) + "|" + strings.TrimSpace(parsed.RunInMode)
	flowShapeCacheMu.Lock()
	if len(flowShapeCache) >= flowShapeCacheLimit {
		flowShapeCache = map[[sha256.Size]byte]string{}
	}
	flowShapeCache[key] = shape
	flowShapeCacheMu.Unlock()
	return shape, true
}

// FlowRunModeRejection names a Flow file that Salesforce rejects at deploy
// time because its run mode needs a later Flow API version.
type FlowRunModeRejection struct {
	File    string
	Message string
}

// flowRunModeShape is the part of a Flow file that decides the rejection.
type flowRunModeShape struct {
	APIVersion string `xml:"apiVersion"`
	RunInMode  string `xml:"runInMode"`
}

// FlowRunModeShapes returns the declared API version and run mode of every
// Flow file that sets runInMode, keyed by path. Flows without runInMode
// cannot be rejected and are omitted, so callers can bind cached results to
// the shapes without hashing every Flow.
func FlowRunModeShapes(p Project, read func(string) ([]byte, error)) map[string]string {
	if len(p.FlowFiles) == 0 {
		return nil
	}
	if read == nil {
		read = p.ReadFile
	}
	var shapes map[string]string
	for _, file := range p.FlowFiles {
		data, err := read(file)
		if err != nil || !bytes.Contains(data, []byte("runInMode")) {
			continue
		}
		shape, ok := flowShapeForContent(data)
		if !ok {
			continue
		}
		if shapes == nil {
			shapes = make(map[string]string)
		}
		shapes[file] = shape
	}
	return shapes
}

// FlowRunModeRejections reports Flows that set runInMode UserMode at a Flow
// API version below 68.0. The API 68 capture (apextest api68_native flow
// FLOW-DEPLOY-67-USERMODE) rejects such a Flow saved at 67.0 and deploys the
// same Flow saved at 68.0 (FLOW-DEPLOY-68). A Flow without an apiVersion
// element was not captured and is not rejected.
func FlowRunModeRejections(p Project, read func(string) ([]byte, error)) []FlowRunModeRejection {
	return FlowRunModeRejectionsFromShapes(p, FlowRunModeShapes(p, read))
}

// FlowRunModeRejectionsFromShapes is FlowRunModeRejections over shapes the
// caller already read with FlowRunModeShapes, so an index build reads each
// Flow file once.
func FlowRunModeRejectionsFromShapes(p Project, shapes map[string]string) []FlowRunModeRejection {
	var rejections []FlowRunModeRejection
	for _, file := range p.FlowFiles {
		shape, ok := shapes[file]
		if !ok {
			continue
		}
		version, mode, _ := strings.Cut(shape, "|")
		major, err := strconv.ParseFloat(version, 64)
		if mode != "UserMode" || err != nil || major >= 68 {
			continue
		}
		name := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(file), "-meta.xml"), ".flow")
		rejections = append(rejections, FlowRunModeRejection{
			File:    file,
			Message: fmt.Sprintf("We can't save the %q flow. The Explicit User Mode run mode requires API version 68 or later. Select API version 68 or later, or choose a different run mode.", name),
		})
	}
	return rejections
}
