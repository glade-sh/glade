package server

import (
	"encoding/xml"
	"net/http"
)

const soapEnvelopeNamespace = "http://schemas.xmlsoap.org/soap/envelope/"

func (s *Server) handleSOAP(w http.ResponseWriter, r *http.Request, parts []string) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	if len(parts) == 3 && parts[0] == "s" {
		s.handleSOAPApex(w, r, parts[1])
		return
	}
	if len(parts) >= 2 && parts[0] == "u" {
		s.handlePartnerSOAP(w, r, parts[1:])
		return
	}
	writeSOAPFault(w, http.StatusInternalServerError, "sf:UNKNOWN_SOAP_ENDPOINT", "unknown SOAP endpoint")
}

func (s *Server) handleBulkV1(w http.ResponseWriter, r *http.Request, parts []string) {
	s.handleBulkV1Routes(w, r, parts)
}

func writeSOAPXML(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(value)
}

func writeSOAPFault(w http.ResponseWriter, status int, code, message string) {
	writeSOAPXML(w, status, soapFaultEnvelope{
		SoapEnv: soapEnvelopeNamespace,
		Body: soapFaultBody{
			Fault: soapFault{
				Code:   code,
				String: message,
			},
		},
	})
}

type soapFaultEnvelope struct {
	XMLName xml.Name      `xml:"soapenv:Envelope"`
	SoapEnv string        `xml:"xmlns:soapenv,attr"`
	Body    soapFaultBody `xml:"soapenv:Body"`
}

type soapFaultBody struct {
	Fault soapFault `xml:"soapenv:Fault"`
}

type soapFault struct {
	Code   string `xml:"faultcode"`
	String string `xml:"faultstring"`
}
