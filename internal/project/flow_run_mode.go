package project

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

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
		var shape flowRunModeShape
		if err := xml.Unmarshal(data, &shape); err != nil {
			continue
		}
		if shapes == nil {
			shapes = make(map[string]string)
		}
		shapes[file] = strings.TrimSpace(shape.APIVersion) + "|" + strings.TrimSpace(shape.RunInMode)
	}
	return shapes
}

// FlowRunModeRejections reports Flows that set runInMode UserMode at a Flow
// API version below 68.0. The API 68 capture (apextest api68_native flow
// FLOW-DEPLOY-67-USERMODE) rejects such a Flow saved at 67.0 and deploys the
// same Flow saved at 68.0 (FLOW-DEPLOY-68). A Flow without an apiVersion
// element was not captured and is not rejected.
func FlowRunModeRejections(p Project, read func(string) ([]byte, error)) []FlowRunModeRejection {
	shapes := FlowRunModeShapes(p, read)
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
