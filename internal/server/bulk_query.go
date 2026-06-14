package server

import (
	"bytes"
	"encoding/csv"
	"net/http"
	"strconv"
)

func (s *Server) handleBulkJobsWithQueryPaging(w http.ResponseWriter, r *http.Request, version string, parts []string) {
	if len(parts) < 1 {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeJSON(w, http.StatusOK, bulkJobsDiscoveryPayload(version))
		return
	}
	switch parts[0] {
	case "query":
		s.handleBulkQueryJobWithPaging(w, r, version, parts[1:])
	case "ingest":
		writeUnsupportedBulkIngestJob(w, r, parts[1:])
	default:
		writeSalesforceError(w, errUnknownEndpoint)
	}
}

func (s *Server) handleBulkQueryJobWithPaging(w http.ResponseWriter, r *http.Request, version string, parts []string) {
	switch {
	case len(parts) == 0:
		if !methodAllowed(r, http.MethodGet, http.MethodPost) {
			writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
			return
		}
		if r.Method == http.MethodPost {
			s.createBulkQueryJob(w, r, version)
			return
		}
		writeSalesforceError(w, errUnsupportedFeature, "Bulk API v2 query jobs collection listing is not implemented in the local server")
	case len(parts) == 1:
		if !methodAllowed(r, http.MethodGet, http.MethodPatch, http.MethodDelete) {
			writeMethodNotAllowed(w, http.MethodGet, http.MethodPatch, http.MethodDelete)
			return
		}
		if r.Method == http.MethodGet {
			s.writeBulkQueryJobRecord(w, parts[0])
			return
		}
		if r.Method == http.MethodPatch {
			if _, ok := decodeOptionalJSONObject(w, r); !ok {
				return
			}
		}
		writeSalesforceError(w, errUnsupportedFeature, "Bulk API v2 query job mutation is not implemented in the local server")
	case len(parts) == 2 && parts[1] == "results":
		if !methodAllowed(r, http.MethodGet) {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		s.writeBulkQueryJobPagedResults(w, r, parts[0])
	default:
		writeSalesforceError(w, errUnknownEndpoint)
	}
}

func (s *Server) writeBulkQueryJobPagedResults(w http.ResponseWriter, r *http.Request, id string) {
	job, ok := s.lookupBulkQueryJob(id)
	if !ok {
		writeSalesforceError(w, errUnknownEndpoint, "Bulk API v2 query job not found")
		return
	}
	start, ok := parseBulkQueryLocator(w, r.URL.Query().Get("locator"), len(job.records))
	if !ok {
		return
	}
	maxRecords, ok := parseBulkQueryMaxRecords(w, r.URL.Query().Get("maxRecords"), len(job.records)-start)
	if !ok {
		return
	}
	end := start + maxRecords
	if end > len(job.records) {
		end = len(job.records)
	}
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	if err := writer.Write(job.fields); err != nil {
		writeSalesforceError(w, errStoreFailure, err.Error())
		return
	}
	for _, record := range job.records[start:end] {
		row := make([]string, 0, len(job.fields))
		for _, field := range job.fields {
			row = append(row, bulkCSVValue(record, field))
		}
		if err := writer.Write(row); err != nil {
			writeSalesforceError(w, errStoreFailure, err.Error())
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		writeSalesforceError(w, errStoreFailure, err.Error())
		return
	}
	locator := "null"
	if end < len(job.records) {
		locator = strconv.Itoa(end)
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Sforce-Locator", locator)
	w.Header().Set("Sforce-NumberOfRecords", strconv.Itoa(end-start))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buffer.Bytes())
}

func parseBulkQueryLocator(w http.ResponseWriter, raw string, total int) (int, bool) {
	if raw == "" {
		return 0, true
	}
	if raw == "null" {
		if total == 0 {
			return 0, true
		}
		writeSalesforceError(w, errMalformedQuery, "invalid bulk query locator")
		return 0, false
	}
	offset, err := strconv.Atoi(raw)
	if err != nil || offset < 0 || offset >= total {
		writeSalesforceError(w, errMalformedQuery, "invalid bulk query locator")
		return 0, false
	}
	return offset, true
}

func parseBulkQueryMaxRecords(w http.ResponseWriter, raw string, fallback int) (int, bool) {
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		writeSalesforceError(w, errMalformedQuery, "maxRecords must be a positive integer")
		return 0, false
	}
	return value, true
}
