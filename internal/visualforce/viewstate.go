package visualforce

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/glade-sh/glade/internal/vm"
)

const viewStateFieldName = "com.salesforce.visualforce.ViewState"
const viewStateActionField = "__vf_action"
const CurrentViewStateVersion = 1
const viewStateEnvelopeHeader = "GLVFS\x01"
const viewStateKeyDomain = "glade.visualforce.viewstate.key.v1\x00"

var (
	ErrViewStateInvalid  = errors.New("invalid view state")
	ErrViewStateTampered = errors.New("view state signature mismatch")
	ErrViewStateExpired  = errors.New("view state expired")
	ErrViewStateCSRF     = errors.New("view state csrf mismatch")
	ErrViewStateEntropy  = errors.New("view state secure randomness unavailable")
)

type viewStateKeyCache struct {
	once sync.Once
	key  []byte
	err  error
}

func (c *viewStateKeyCache) get(reader io.Reader) ([]byte, error) {
	c.once.Do(func() {
		key, err := readViewStateRandom(reader, 32)
		if err != nil {
			c.err = fmt.Errorf("%w: generate process key: %v", ErrViewStateEntropy, err)
			return
		}
		c.key = key
	})
	if c.err != nil {
		return nil, c.err
	}
	return append([]byte(nil), c.key...), nil
}

var processViewStateKey viewStateKeyCache

func readViewStateRandom(reader io.Reader, size int) ([]byte, error) {
	value := make([]byte, size)
	if _, err := io.ReadFull(reader, value); err != nil {
		for i := range value {
			value[i] = 0
		}
		return nil, fmt.Errorf("%w: read random bytes: %v", ErrViewStateEntropy, err)
	}
	return value, nil
}

func viewStateAEAD(secret []byte) (cipher.AEAD, error) {
	if secret == nil {
		var err error
		secret, err = processViewStateKey.get(rand.Reader)
		if err != nil {
			return nil, err
		}
	} else if len(secret) == 0 {
		return nil, fmt.Errorf("%w: explicit secret is empty", ErrViewStateInvalid)
	}

	keyHash := sha256.New()
	_, _ = keyHash.Write([]byte(viewStateKeyDomain))
	_, _ = keyHash.Write(secret)
	block, err := aes.NewCipher(keyHash.Sum(nil))
	if err != nil {
		return nil, fmt.Errorf("%w: initialize view state cipher", ErrViewStateInvalid)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: initialize view state authentication", ErrViewStateInvalid)
	}
	return aead, nil
}

type ViewStatePayload struct {
	Version          int                   `json:"v,omitempty"`
	PageName         string                `json:"pn"`
	CSRF             string                `json:"csrf"`
	Timestamp        int64                 `json:"ts"`
	ControllerType   string                `json:"ct,omitempty"`
	ControllerState  json.RawMessage       `json:"cstate,omitempty"`
	ControllerValues map[string]vm.Value   `json:"cv,omitempty"`
	ControllerFields map[string]string     `json:"cf,omitempty"`
	ExtensionValues  []map[string]vm.Value `json:"ev,omitempty"`
	ExtensionFields  []map[string]string   `json:"ef,omitempty"`
	ExtensionState   []json.RawMessage     `json:"estate,omitempty"`
	PageMessages     []string              `json:"pm,omitempty"`
	ComponentState   map[string]string     `json:"cs,omitempty"`
}

func ViewStateFormFieldName() string {
	return viewStateFieldName
}

func ViewStateActionFieldName() string {
	return viewStateActionField
}

func EncodeViewState(payload ViewStatePayload, secret []byte) (string, error) {
	return encodeViewStateWithRandom(payload, secret, rand.Reader)
}

func encodeViewStateWithRandom(payload ViewStatePayload, secret []byte, reader io.Reader) (string, error) {
	aead, err := viewStateAEAD(secret)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.CSRF) == "" {
		token, err := randomToken(16)
		if err != nil {
			return "", err
		}
		payload.CSRF = token
	}
	if payload.Version == 0 {
		payload.Version = CurrentViewStateVersion
	}
	if payload.Timestamp == 0 {
		payload.Timestamp = time.Now().Unix()
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	nonce, err := readViewStateRandom(reader, aead.NonceSize())
	if err != nil {
		return "", fmt.Errorf("%w: generate nonce: %v", ErrViewStateEntropy, err)
	}
	header := []byte(viewStateEnvelopeHeader)
	ciphertext := aead.Seal(nil, nonce, raw, header)
	envelope := make([]byte, 0, len(header)+len(nonce)+len(ciphertext))
	envelope = append(envelope, header...)
	envelope = append(envelope, nonce...)
	envelope = append(envelope, ciphertext...)
	return base64.StdEncoding.EncodeToString(envelope), nil
}

func DecodeViewState(encoded string, secret []byte) (ViewStatePayload, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return ViewStatePayload{}, fmt.Errorf("%w: decode failed", ErrViewStateInvalid)
	}
	aead, err := viewStateAEAD(secret)
	if err != nil {
		return ViewStatePayload{}, err
	}
	header := []byte(viewStateEnvelopeHeader)
	minimumSize := len(header) + aead.NonceSize() + aead.Overhead()
	if len(raw) < minimumSize {
		return ViewStatePayload{}, fmt.Errorf("%w: envelope too short", ErrViewStateInvalid)
	}
	if string(raw[:len(header)]) != viewStateEnvelopeHeader {
		return ViewStatePayload{}, fmt.Errorf("%w: unsupported envelope version", ErrViewStateInvalid)
	}
	nonceStart := len(header)
	nonceEnd := nonceStart + aead.NonceSize()
	payloadBytes, err := aead.Open(nil, raw[nonceStart:nonceEnd], raw[nonceEnd:], raw[:len(header)])
	if err != nil {
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
		return ErrViewStateCSRF
	}
	if strings.TrimSpace(formCSRF) != payload.CSRF {
		return ErrViewStateCSRF
	}
	return nil
}

func ensureViewStateCSRF(payload *ViewStatePayload) error {
	if payload == nil || strings.TrimSpace(payload.CSRF) != "" {
		return nil
	}
	token, err := randomToken(16)
	if err != nil {
		return err
	}
	payload.CSRF = token
	return nil
}

func InjectCSRF(html string, csrf string) string {
	if strings.TrimSpace(csrf) == "" || strings.Contains(html, `name="__vf_csrf"`) {
		return html
	}
	field := `<input type="hidden" name="__vf_csrf" value="` + htmlAttrEscape(csrf) + `" />`
	if strings.Contains(html, "</form>") {
		return strings.ReplaceAll(html, "</form>", field+"</form>")
	}
	if strings.Contains(html, "</body>") {
		return strings.Replace(html, "</body>", field+"</body>", 1)
	}
	return html + field
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("%w: generate token: %v", ErrViewStateEntropy, err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func InjectViewState(html string, viewState string) string {
	if strings.TrimSpace(viewState) == "" {
		return html
	}
	field := `<input type="hidden" name="` + viewStateFieldName + `" value="` + htmlAttrEscape(viewState) + `" />`
	if strings.Contains(html, "</form>") {
		return strings.ReplaceAll(html, "</form>", field+"</form>")
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
