package visualforce

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInputFileUploadBindingCapturesBlobAndMetadata(t *testing.T) {
	req := newMultipartUploadRequest(t, "f:file", "invoice.txt", "text/plain", []byte("hello"))

	upload, err := BindInputFileUpload(req, "f:file")
	if err != nil {
		t.Fatalf("BindInputFileUpload err = %v", err)
	}
	if upload.FileName != "invoice.txt" || upload.ContentType != "text/plain" || upload.Size != 5 {
		t.Fatalf("upload metadata = %#v", upload)
	}
	if string(upload.Blob) != "hello" {
		t.Fatalf("blob = %q", string(upload.Blob))
	}

	viewState, err := json.Marshal(upload)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(viewState, []byte("hello")) {
		t.Fatalf("view state serialized blob body: %s", string(viewState))
	}
}

func TestInputFileUploadBindingsFromMarkupCaptureBlobAndMetadataTargets(t *testing.T) {
	bindings, err := InputFileUploadBindingsFromMarkup(`<apex:page>
  <apex:form>
    <apex:inputFile id="upload" value="{!body}" fileName="{!fileName}" contentType="{!mimeType}" fileSize="{!byteCount}"/>
  </apex:form>
</apex:page>`)
	if err != nil {
		t.Fatalf("InputFileUploadBindingsFromMarkup err = %v", err)
	}
	if len(bindings) != 1 {
		t.Fatalf("bindings = %#v", bindings)
	}
	binding := bindings[0]
	if binding.FieldName != "upload" || binding.BlobTarget != "body" || binding.FileNameTarget != "fileName" || binding.ContentTypeTarget != "mimeType" || binding.SizeTarget != "byteCount" {
		t.Fatalf("binding = %#v", binding)
	}
}

func TestInputFileUploadAssignmentsBindBlobFilenameContentTypeAndSize(t *testing.T) {
	req := newMultipartUploadRequest(t, "upload", "invoice.txt", "text/plain", []byte("hello"))
	assignments, err := BindInputFileUploadAssignments(req, []InputFileUploadBinding{{
		FieldName:         "upload",
		BlobTarget:        "body",
		FileNameTarget:    "fileName",
		ContentTypeTarget: "mimeType",
		SizeTarget:        "byteCount",
	}})
	if err != nil {
		t.Fatalf("BindInputFileUploadAssignments err = %v", err)
	}
	if string(assignments["body"].([]byte)) != "hello" {
		t.Fatalf("body = %#v", assignments["body"])
	}
	if assignments["fileName"] != "invoice.txt" || assignments["mimeType"] != "text/plain" || assignments["byteCount"] != int64(5) {
		t.Fatalf("assignments = %#v", assignments)
	}
}

func TestRenderInputFileUsesVisualforceIDForFileControl(t *testing.T) {
	tree, err := ParseMarkupTree(`<apex:page>
  <apex:form>
    <apex:inputFile id="upload" value="{!body}"/>
  </apex:form>
</apex:page>`)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := RenderMarkupTree(tree, &RenderContext{PageName: "Upload", Expression: &ExpressionContext{}})
	if err != nil {
		t.Fatalf("RenderMarkupTree err = %v", err)
	}
	for _, want := range []string{
		`enctype="multipart/form-data"`,
		`<input type="file"`,
		`name="upload"`,
		`id="upload"`,
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered missing %q: %s", want, rendered)
		}
	}
}

func TestInputFileRegistryStatusIsPartialWithUploadGapReason(t *testing.T) {
	spec, ok := StandardComponentSpec("apex", "inputFile")
	if !ok {
		t.Fatal("missing registry spec for apex:inputFile")
	}
	if spec.Status != ComponentPartial {
		t.Fatalf("status = %s, want %s", spec.Status, ComponentPartial)
	}
	if spec.Render == nil {
		t.Fatal("renderer is nil")
	}
	reason := strings.ToLower(spec.Reason)
	for _, want := range []string{"file input", "blob"} {
		if !strings.Contains(reason, want) {
			t.Fatalf("reason = %q, want it to mention %q", spec.Reason, want)
		}
	}
}

func TestInputFileUploadRejectsOversizedBody(t *testing.T) {
	req := newMultipartUploadRequest(t, "f:file", "big.bin", "application/octet-stream", bytes.Repeat([]byte("x"), MaxVisualforceUploadBytes+1))

	_, err := BindInputFileUpload(req, "f:file")
	if err == nil || !strings.Contains(err.Error(), "visualforce upload limit exceeded") {
		t.Fatalf("err = %v, want upload limit diagnostic", err)
	}
}

func newMultipartUploadRequest(t *testing.T, fieldName, fileName, contentType string, body []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreatePart(map[string][]string{
		"Content-Disposition": {`form-data; name="` + fieldName + `"; filename="` + fileName + `"`},
		"Content-Type":        {contentType},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/apex/Upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.ContentLength = int64(buf.Len())
	return req
}
