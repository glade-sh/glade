package server

import (
	"bytes"
	"image/gif"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// V15 dom_styles_{default,true}_header_{true,false} and
// dom_layout_pageBlockSection_{normal,empty,pass} back these asset contracts.
func TestVisualforcePresentationAssets(t *testing.T) {
	handler := &Server{}
	css := httptest.NewRecorder()
	handler.ServeHTTP(css, httptest.NewRequest(http.MethodGet, "/styles/glade-visualforce.css", nil))
	if css.Code != http.StatusOK || !strings.HasPrefix(css.Header().Get("Content-Type"), "text/css") || !strings.Contains(css.Body.String(), "rgb(34, 34, 34)") {
		t.Fatalf("standard stylesheet response: status %d, type %q, body %q", css.Code, css.Header().Get("Content-Type"), css.Body.String())
	}
	spacer := httptest.NewRecorder()
	handler.ServeHTTP(spacer, httptest.NewRequest(http.MethodGet, "/img/s.gif", nil))
	if spacer.Code != http.StatusOK || spacer.Header().Get("Content-Type") != "image/gif" {
		t.Fatalf("spacer response: status %d, type %q", spacer.Code, spacer.Header().Get("Content-Type"))
	}
	img, err := gif.Decode(bytes.NewReader(spacer.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 1 || img.Bounds().Dy() != 1 {
		t.Fatalf("spacer size %v, want one pixel", img.Bounds())
	}
}
