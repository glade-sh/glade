package visualforce

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const viewStateFieldName = "com.salesforce.visualforce.ViewState"
const viewStateActionField = "__vf_action"

var (
	ErrViewStateInvalid   = errors.New("invalid view state")
	ErrViewStateTampered  = errors.New("view state signature mismatch")
	ErrViewStateExpired   = errors.New("view state expired")
	ErrViewStateCSRF      = errors.New("view state csrf mismatch")
	defaultViewStateSecret = []byte("glade-local-vf-viewstate")
)

type ViewStatePayload struct {
	PageName         string            `json:"pn"`
	CSRF             string            `json:"csrf"`
	Timestamp        int64             `json:"ts"`
	ControllerType   string            `json:"ct,omitempty"`
	ControllerFields map[string]string `json:"cf,omitempty"`
	ExtensionFields  []map[string]string `json:"ef,omitempty"`
	PageMessages     []string          `json:"pm,omitempty"`
	ComponentState   map[string]string `json:"cs,omitempty"`
}

func ViewStateFormFieldName() string {
	return viewStateFieldName
}

func ViewStateActionFieldName() string {
	return viewStateActionField
}

func EncodeViewState(payload ViewStatePayload, secret []byte) (string, error) {
	if secret == nil {
		secret = defaultViewStateSecret
	}
	if strings.TrimSpace(payload.CSRF) == "" {
		token, err := randomToken(16)
		if err != nil {
			return "", err
		}
		payload.CSRF = token
	}
	if payload.Timestamp == 0 {
		payload.Timestamp = time.Now().Unix()
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(raw)
	sig := mac.Sum(nil)
	combined := append(raw, sig...)
	return base64.StdEncoding.EncodeToString(combined), nil
}

func DecodeViewState(encoded string, secret []byte) (ViewStatePayload, error) {
	if secret == nil {
		secret = defaultViewStateSecret
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return ViewStatePayload{}, fmt.Errorf("%w: decode failed", ErrViewStateInvalid)
	}
	if len(raw) < sha256.Size {
		return ViewStatePayload{}, fmt.Errorf("%w: payload too short", ErrViewStateInvalid)
	}
	payloadBytes := raw[:len(raw)-sha256.Size]
	sig := raw[len(raw)-sha256.Size:]
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(payloadBytes)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return ViewStatePayload{}, ErrViewStateTampered
	}
	var payload ViewStatePayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return ViewStatePayload{}, fmt.Errorf("%w: %v", ErrViewStateInvalid, err)
	}
	if payload.Timestamp > 0 && time.Now().Unix()-payload.Timestamp > 24*3600 {
		return ViewStatePayload{}, ErrViewStateExpired
	}
	return payload, nil
}

func VerifyViewStateCSRF(payload ViewStatePayload, formCSRF string) error {
	if strings.TrimSpace(payload.CSRF) == "" {
		return nil
	}
	if strings.TrimSpace(formCSRF) != payload.CSRF {
		return ErrViewStateCSRF
	}
	return nil
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func InjectViewState(html string, viewState string) string {
	if strings.TrimSpace(viewState) == "" {
		return html
	}
	field := `<input type="hidden" name="` + viewStateFieldName + `" value="` + htmlAttrEscape(viewState) + `" />`
	if strings.Contains(html, "</form>") {
		return strings.Replace(html, "</form>", field+"</form>", 1)
	}
	if strings.Contains(html, "</body>") {
		return strings.Replace(html, "</body>", field+"</body>", 1)
	}
	return html + field
}

func htmlAttrEscape(raw string) string {
	raw = strings.ReplaceAll(raw, "&", "&amp;")
	raw = strings.ReplaceAll(raw, `"`, "&quot;")
	return raw
}
