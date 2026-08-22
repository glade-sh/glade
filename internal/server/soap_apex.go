package server

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
)

type soapParseErrorKind int

const (
	soapParseMalformed soapParseErrorKind = iota
	soapParseUnknownMethod
)

type soapParseError struct {
	kind    soapParseErrorKind
	message string
}

func (e soapParseError) Error() string {
	return e.message
}

type toolingExecuteAnonymousSOAPResult struct {
	Compiled            bool    `json:"compiled"`
	Success             bool    `json:"success"`
	Line                int     `json:"line"`
	Column              int     `json:"column"`
	CompileProblem      *string `json:"compileProblem"`
	ExceptionMessage    *string `json:"exceptionMessage"`
	ExceptionStackTrace *string `json:"exceptionStackTrace"`
}

func (s *Server) handleSOAPApex(w http.ResponseWriter, r *http.Request, apiVersion string) {
	source, err := parseSOAPExecuteAnonymous(r.Body)
	if err != nil {
		var parseErr soapParseError
		if errors.As(err, &parseErr) && parseErr.kind == soapParseUnknownMethod {
			writeSOAPFault(w, http.StatusInternalServerError, "sf:UNKNOWN_SOAP_METHOD", err.Error())
			return
		}
		writeSOAPFault(w, http.StatusInternalServerError, "sf:MALFORMED_XML", err.Error())
		return
	}

	result, err := s.runToolingExecuteAnonymousForSOAP(r, source, apiVersion)
	if err != nil {
		writeSOAPFault(w, http.StatusInternalServerError, "sf:SERVER", err.Error())
		return
	}
	writeSOAPXML(w, http.StatusOK, newSOAPApexEnvelope(result))
}

func parseSOAPExecuteAnonymous(body io.Reader) (string, error) {
	decoder := xml.NewDecoder(body)
	inBody := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", soapParseError{kind: soapParseMalformed, message: "malformed SOAP XML: " + err.Error()}
		}
		switch token := token.(type) {
		case xml.StartElement:
			if token.Name.Local == "Body" {
				inBody = true
				continue
			}
			if !inBody {
				continue
			}
			if token.Name.Local != "executeAnonymous" {
				return "", soapParseError{kind: soapParseUnknownMethod, message: "unknown SOAP method: " + token.Name.Local}
			}
			return parseExecuteAnonymousString(decoder, token)
		case xml.EndElement:
			if token.Name.Local == "Body" {
				inBody = false
			}
		}
	}
	return "", soapParseError{kind: soapParseUnknownMethod, message: "unknown SOAP method"}
}

func parseExecuteAnonymousString(decoder *xml.Decoder, method xml.StartElement) (string, error) {
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return "", soapParseError{kind: soapParseMalformed, message: "malformed SOAP XML: unexpected end of executeAnonymous"}
		}
		if err != nil {
			return "", soapParseError{kind: soapParseMalformed, message: "malformed SOAP XML: " + err.Error()}
		}
		switch token := token.(type) {
		case xml.StartElement:
			if token.Name.Local != "String" && token.Name.Local != "apexcode" {
				if err := decoder.Skip(); err != nil {
					return "", soapParseError{kind: soapParseMalformed, message: "malformed SOAP XML: " + err.Error()}
				}
				continue
			}
			var source string
			if err := decoder.DecodeElement(&source, &token); err != nil {
				return "", soapParseError{kind: soapParseMalformed, message: "malformed SOAP XML: " + err.Error()}
			}
			return source, nil
		case xml.EndElement:
			if token.Name.Local == method.Name.Local {
				return "", soapParseError{kind: soapParseMalformed, message: "executeAnonymous source is required"}
			}
		}
	}
}

func (s *Server) runToolingExecuteAnonymousForSOAP(r *http.Request, source, apiVersion string) (toolingExecuteAnonymousSOAPResult, error) {
	body, err := json.Marshal(map[string]string{"anonymousBody": source})
	if err != nil {
		return toolingExecuteAnonymousSOAPResult{}, err
	}
	req := httptest.NewRequest(http.MethodPost, "/services/data/v"+apiVersion+"/tooling/executeAnonymous", bytes.NewReader(body))
	req.Header = r.Header.Clone()
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	s.handleExecuteAnonymous(rec, req, apiVersion)
	if rec.Code != http.StatusOK {
		return toolingExecuteAnonymousSOAPResult{}, soapToolingError(rec.Body.Bytes())
	}
	var result toolingExecuteAnonymousSOAPResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		return toolingExecuteAnonymousSOAPResult{}, err
	}
	return result, nil
}

func soapToolingError(body []byte) error {
	var payload []salesforceError
	if err := json.Unmarshal(body, &payload); err == nil && len(payload) > 0 {
		return errors.New(payload[0].Message)
	}
	return errors.New(string(body))
}

func newSOAPApexEnvelope(result toolingExecuteAnonymousSOAPResult) soapApexEnvelope {
	return soapApexEnvelope{
		SoapEnv: soapEnvelopeNamespace,
		Body: soapApexBody{
			Response: soapApexExecuteAnonymousResponse{
				Xmlns: "urn:partner.soap.sforce.com",
				Result: soapApexExecuteAnonymousResult{
					Compiled:            result.Compiled,
					Success:             result.Success,
					Line:                result.Line,
					Column:              result.Column,
					CompileProblem:      stringValue(result.CompileProblem),
					ExceptionMessage:    stringValue(result.ExceptionMessage),
					ExceptionStackTrace: stringValue(result.ExceptionStackTrace),
				},
			},
		},
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

type soapApexEnvelope struct {
	XMLName xml.Name     `xml:"soapenv:Envelope"`
	SoapEnv string       `xml:"xmlns:soapenv,attr"`
	Body    soapApexBody `xml:"soapenv:Body"`
}

type soapApexBody struct {
	Response soapApexExecuteAnonymousResponse `xml:"executeAnonymousResponse"`
}

type soapApexExecuteAnonymousResponse struct {
	Xmlns  string                         `xml:"xmlns,attr"`
	Result soapApexExecuteAnonymousResult `xml:"result"`
}

type soapApexExecuteAnonymousResult struct {
	Compiled            bool   `xml:"compiled"`
	Success             bool   `xml:"success"`
	Line                int    `xml:"line"`
	Column              int    `xml:"column"`
	CompileProblem      string `xml:"compileProblem"`
	ExceptionMessage    string `xml:"exceptionMessage"`
	ExceptionStackTrace string `xml:"exceptionStackTrace"`
}
