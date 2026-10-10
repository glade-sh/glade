package visualforce

import (
	"errors"
	"testing"
	"time"
)

// Preserve the preexisting local expiry policy; this is not an assertion
// about Salesforce's proprietary envelope or its expiry interval.
func TestDecodeViewStatePreservesExpiryPolicy(t *testing.T) {
	for _, tc := range []struct {
		name string
		age  time.Duration
		want error
	}{
		{"recent", time.Minute, nil},
		{"expired", 25 * time.Hour, ErrViewStateExpired},
	} {
		t.Run(tc.name, func(t *testing.T) {
			secret := []byte("owned-expiry-control-key")
			encoded, err := EncodeViewState(ViewStatePayload{
				PageName: "Edit", CSRF: "expiry-csrf", Timestamp: time.Now().Add(-tc.age).Unix(),
			}, secret)
			if err != nil {
				t.Fatal(err)
			}
			_, err = DecodeViewState(encoded, secret)
			if !errors.Is(err, tc.want) {
				t.Fatalf("decode error=%v want=%v", err, tc.want)
			}
		})
	}
}

func TestVerifyViewStateCSRFPreservesMatchAndMismatch(t *testing.T) {
	payload := ViewStatePayload{CSRF: "owned-token"}
	for _, tc := range []struct {
		name, token string
		want        error
	}{
		{"match", "owned-token", nil},
		{"wrong", "wrong-token", ErrViewStateCSRF},
		{"empty", "", ErrViewStateCSRF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := VerifyViewStateCSRF(payload, tc.token); !errors.Is(err, tc.want) {
				t.Fatalf("CSRF error=%v want=%v", err, tc.want)
			}
		})
	}
}
