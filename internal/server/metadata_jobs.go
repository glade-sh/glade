package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type metadataLocalJob struct {
	ID         string
	Kind       string
	CheckOnly  bool
	Components []metadataComponent
}

func (s *Server) handleMetadataRESTWithJobs(w http.ResponseWriter, r *http.Request, version string, parts []string) {
	if len(parts) >= 1 {
		switch parts[0] {
		case "retrieveRequest":
			s.handleMetadataRetrieveJob(w, r, parts[1:])
			return
		case "deployRequest":
			s.handleMetadataDeployJob(w, r, parts[1:])
			return
		}
	}
	s.handleMetadataREST(w, r, version, parts)
}

func (s *Server) handleMetadataRetrieveJob(w http.ResponseWriter, r *http.Request, parts []string) {
	switch {
	case len(parts) == 0:
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		if _, ok := decodeOptionalJSONObject(w, r); !ok {
			return
		}
		job := s.createMetadataJob("retrieve", false)
		writeJSON(w, http.StatusOK, metadataJobPayload(job))
	case len(parts) == 1:
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		job, ok := s.lookupMetadataJob(parts[0], "retrieve")
		if !ok {
			writeSalesforceError(w, errUnknownEndpoint, "metadata job not found")
			return
		}
		writeJSON(w, http.StatusOK, metadataJobPayload(job))
	case len(parts) == 2 && parts[1] == "results":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		job, ok := s.lookupMetadataJob(parts[0], "retrieve")
		if !ok {
			writeSalesforceError(w, errUnknownEndpoint, "metadata job not found")
			return
		}
		writeJSON(w, http.StatusOK, metadataRetrieveResultsPayload(job))
	default:
		writeSalesforceError(w, errUnknownEndpoint)
	}
}

func (s *Server) handleMetadataDeployJob(w http.ResponseWriter, r *http.Request, parts []string) {
	switch {
	case len(parts) == 0:
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		body, ok := decodeOptionalJSONObject(w, r)
		if !ok {
			return
		}
		job := s.createMetadataJob("deploy", metadataDeployCheckOnly(body))
		writeJSON(w, http.StatusOK, metadataJobPayload(job))
	case len(parts) == 1:
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		job, ok := s.lookupMetadataJob(parts[0], "deploy")
		if !ok {
			writeSalesforceError(w, errUnknownEndpoint, "metadata job not found")
			return
		}
		writeJSON(w, http.StatusOK, metadataJobPayload(job))
	case len(parts) == 2 && (parts[1] == "results" || parts[1] == "deployDetails"):
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		job, ok := s.lookupMetadataJob(parts[0], "deploy")
		if !ok {
			writeSalesforceError(w, errUnknownEndpoint, "metadata job not found")
			return
		}
		if parts[1] == "deployDetails" {
			writeJSON(w, http.StatusOK, metadataDeployDetailsPayload(job))
			return
		}
		writeJSON(w, http.StatusOK, metadataDeployResultsPayload(job))
	default:
		writeSalesforceError(w, errUnknownEndpoint)
	}
}

func (s *Server) createMetadataJob(kind string, checkOnly bool) metadataLocalJob {
	if s.metadataJobs == nil {
		s.metadataJobs = make(map[string]metadataLocalJob)
	}
	id := ""
	switch kind {
	case "deploy":
		s.nextMetadataDeployID++
		id = fmt.Sprintf("0Af%012d", s.nextMetadataDeployID)
	case "retrieve":
		s.nextMetadataRetrieveID++
		id = fmt.Sprintf("0Ar%012d", s.nextMetadataRetrieveID)
	}
	job := metadataLocalJob{
		ID:         id,
		Kind:       kind,
		CheckOnly:  checkOnly,
		Components: append([]metadataComponent(nil), s.Source.Components...),
	}
	s.metadataJobs[id] = job
	return job
}

func (s *Server) lookupMetadataJob(id, kind string) (metadataLocalJob, bool) {
	job, ok := s.metadataJobs[id]
	if !ok || job.Kind != kind {
		return metadataLocalJob{}, false
	}
	return job, true
}

func metadataJobPayload(job metadataLocalJob) map[string]any {
	payload := map[string]any{
		"id":                    job.ID,
		"done":                  true,
		"status":                "Succeeded",
		"success":               true,
		"kind":                  job.Kind,
		"numberComponentsTotal": len(job.Components),
		"componentErrors":       []any{},
	}
	if job.Kind == "deploy" {
		payload["checkOnly"] = job.CheckOnly
		payload["numberComponentsDeployed"] = len(job.Components)
	}
	return payload
}

func metadataRetrieveResultsPayload(job metadataLocalJob) map[string]any {
	components := make([]map[string]any, 0, len(job.Components))
	for _, component := range job.Components {
		components = append(components, metadataFileProperties(component))
	}
	return map[string]any{
		"id":         job.ID,
		"done":       true,
		"status":     "Succeeded",
		"success":    true,
		"components": components,
	}
}

func metadataDeployResultsPayload(job metadataLocalJob) map[string]any {
	return map[string]any{
		"id":                 job.ID,
		"done":               true,
		"status":             "Succeeded",
		"success":            true,
		"componentSuccesses": metadataDeployComponentRows(job),
		"componentFailures":  []any{},
		"checkOnly":          job.CheckOnly,
	}
}

func metadataDeployDetailsPayload(job metadataLocalJob) map[string]any {
	return map[string]any{
		"id":      job.ID,
		"details": metadataDeployResultsPayload(job),
	}
}

func metadataDeployComponentRows(job metadataLocalJob) []map[string]any {
	rows := make([]map[string]any, 0, len(job.Components))
	for _, component := range job.Components {
		rows = append(rows, map[string]any{
			"componentType": component.Type,
			"fullName":      component.FullName,
			"fileName":      component.FileName,
			"success":       true,
			"changed":       false,
			"created":       false,
			"deleted":       false,
		})
	}
	return rows
}

func metadataDeployCheckOnly(body map[string]json.RawMessage) bool {
	raw, ok := body["deployOptions"]
	if !ok {
		return false
	}
	var options struct {
		CheckOnly bool `json:"checkOnly"`
	}
	if err := json.Unmarshal(raw, &options); err != nil {
		return false
	}
	return options.CheckOnly
}
