package visualforce

import (
	"errors"
	"testing"
	"time"
)

func TestVisualforceLimitConstantsMatchDocs(t *testing.T) {
	if MaxVisualforceResponseBytes != 15*1000*1000 {
		t.Fatalf("MaxVisualforceResponseBytes = %d", MaxVisualforceResponseBytes)
	}
	if MaxVisualforceViewStateBytes != 170*1024 {
		t.Fatalf("MaxVisualforceViewStateBytes = %d", MaxVisualforceViewStateBytes)
	}
	if MaxVisualforceEmailTemplateBytes != 1*1000*1000 {
		t.Fatalf("MaxVisualforceEmailTemplateBytes = %d", MaxVisualforceEmailTemplateBytes)
	}
	if MaxVisualforceUploadBytes != 10*1000*1000 {
		t.Fatalf("MaxVisualforceUploadBytes = %d", MaxVisualforceUploadBytes)
	}
	if MaxVisualforcePDFHTMLResponseBytes != 15*1000*1000 {
		t.Fatalf("MaxVisualforcePDFHTMLResponseBytes = %d", MaxVisualforcePDFHTMLResponseBytes)
	}
	if MaxVisualforcePDFBytes != 60*1000*1000 {
		t.Fatalf("MaxVisualforcePDFBytes = %d", MaxVisualforcePDFBytes)
	}
	if MaxVisualforcePDFImageBytes != 30*1000*1000 {
		t.Fatalf("MaxVisualforcePDFImageBytes = %d", MaxVisualforcePDFImageBytes)
	}
	if MaxVisualforceHeaderBytes != 8192 {
		t.Fatalf("MaxVisualforceHeaderBytes = %d", MaxVisualforceHeaderBytes)
	}
	if MaxVisualforceRemotingRequestBytes != 4*1000*1000 {
		t.Fatalf("MaxVisualforceRemotingRequestBytes = %d", MaxVisualforceRemotingRequestBytes)
	}
	if DefaultVisualforceRemotingTimeout != 30*time.Second {
		t.Fatalf("DefaultVisualforceRemotingTimeout = %s", DefaultVisualforceRemotingTimeout)
	}
	if MaxVisualforceRemotingTimeout != 120*time.Second {
		t.Fatalf("MaxVisualforceRemotingTimeout = %s", MaxVisualforceRemotingTimeout)
	}
	if MaxVisualforceQueryRows != 50000 {
		t.Fatalf("MaxVisualforceQueryRows = %d", MaxVisualforceQueryRows)
	}
	if MaxVisualforceReadOnlyQueryRows != 1000000 {
		t.Fatalf("MaxVisualforceReadOnlyQueryRows = %d", MaxVisualforceReadOnlyQueryRows)
	}
	if MaxVisualforceIterationItems != 1000 {
		t.Fatalf("MaxVisualforceIterationItems = %d", MaxVisualforceIterationItems)
	}
	if MaxVisualforceReadOnlyIterationItems != 10000 {
		t.Fatalf("MaxVisualforceReadOnlyIterationItems = %d", MaxVisualforceReadOnlyIterationItems)
	}
	if MaxVisualforceStandardSetControllerRecords != 10000 {
		t.Fatalf("MaxVisualforceStandardSetControllerRecords = %d", MaxVisualforceStandardSetControllerRecords)
	}
	if MaxVisualforcePageFieldSets != 50 {
		t.Fatalf("MaxVisualforcePageFieldSets = %d", MaxVisualforcePageFieldSets)
	}
	if MaxVisualforceSObjectFieldSets != 2000 {
		t.Fatalf("MaxVisualforceSObjectFieldSets = %d", MaxVisualforceSObjectFieldSets)
	}
	if MaxVisualforceFieldSetLookupFields != 25 {
		t.Fatalf("MaxVisualforceFieldSetLookupFields = %d", MaxVisualforceFieldSetLookupFields)
	}
}

func TestVisualforceLimitChecksReturnStableErrors(t *testing.T) {
	checks := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "view state",
			err:  CheckVisualforceViewStateSize(MaxVisualforceViewStateBytes + 1),
			want: "visualforce view state limit exceeded: 174081 bytes > 174080 bytes",
		},
		{
			name: "upload",
			err:  CheckVisualforceUploadSize(MaxVisualforceUploadBytes + 1),
			want: "visualforce upload limit exceeded: 10000001 bytes > 10000000 bytes",
		},
		{
			name: "remoting",
			err:  CheckVisualforceRemotingRequestSize(MaxVisualforceRemotingRequestBytes + 1),
			want: "visualforce remoting request limit exceeded: 4000001 bytes > 4000000 bytes",
		},
		{
			name: "header",
			err:  CheckVisualforceHeaderSize(MaxVisualforceHeaderBytes + 1),
			want: "visualforce header limit exceeded: 8193 bytes > 8192 bytes",
		},
		{
			name: "response",
			err:  CheckVisualforceResponseSize(MaxVisualforceResponseBytes),
			want: "visualforce response limit exceeded: 15000000 bytes >= 15000000 bytes",
		},
		{
			name: "pdf html response",
			err:  CheckVisualforcePDFHTMLResponseSize(MaxVisualforcePDFHTMLResponseBytes),
			want: "visualforce pdf html response limit exceeded: 15000000 bytes >= 15000000 bytes",
		},
		{
			name: "pdf",
			err:  CheckVisualforcePDFSize(MaxVisualforcePDFBytes + 1),
			want: "visualforce pdf limit exceeded: 60000001 bytes > 60000000 bytes",
		},
		{
			name: "pdf images",
			err:  CheckVisualforcePDFImageSize(MaxVisualforcePDFImageBytes + 1),
			want: "visualforce pdf image limit exceeded: 30000001 bytes > 30000000 bytes",
		},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if !errors.Is(check.err, ErrVisualforceLimitExceeded) {
				t.Fatalf("err = %#v, want ErrVisualforceLimitExceeded", check.err)
			}
			if check.err.Error() != check.want {
				t.Fatalf("err = %q, want %q", check.err.Error(), check.want)
			}
		})
	}
}

func TestVisualforceLimitChecksAllowBoundaryValues(t *testing.T) {
	checks := []error{
		CheckVisualforceViewStateSize(MaxVisualforceViewStateBytes),
		CheckVisualforceUploadSize(MaxVisualforceUploadBytes),
		CheckVisualforceRemotingRequestSize(MaxVisualforceRemotingRequestBytes),
		CheckVisualforceHeaderSize(MaxVisualforceHeaderBytes),
		CheckVisualforceResponseSize(MaxVisualforceResponseBytes - 1),
		CheckVisualforcePDFHTMLResponseSize(MaxVisualforcePDFHTMLResponseBytes - 1),
		CheckVisualforcePDFSize(MaxVisualforcePDFBytes),
		CheckVisualforcePDFImageSize(MaxVisualforcePDFImageBytes),
	}
	for _, err := range checks {
		if err != nil {
			t.Fatalf("boundary check returned %v", err)
		}
	}
}

func TestVisualforceRequestLimitsHonorReadOnlyMode(t *testing.T) {
	normal := VisualforceRequestLimits(false)
	if normal.QueryRows != MaxVisualforceQueryRows {
		t.Fatalf("normal query rows = %d", normal.QueryRows)
	}
	if normal.IterationItems != MaxVisualforceIterationItems {
		t.Fatalf("normal iteration items = %d", normal.IterationItems)
	}

	readOnly := VisualforceRequestLimits(true)
	if readOnly.QueryRows != MaxVisualforceReadOnlyQueryRows {
		t.Fatalf("read-only query rows = %d", readOnly.QueryRows)
	}
	if readOnly.IterationItems != MaxVisualforceReadOnlyIterationItems {
		t.Fatalf("read-only iteration items = %d", readOnly.IterationItems)
	}
}

func TestVisualforceIterationLimitChecksHonorReadOnlyMode(t *testing.T) {
	if err := CheckVisualforceIterationItems(MaxVisualforceIterationItems, false); err != nil {
		t.Fatalf("normal boundary err = %v", err)
	}
	normalErr := CheckVisualforceIterationItems(MaxVisualforceIterationItems+1, false)
	if !errors.Is(normalErr, ErrVisualforceLimitExceeded) {
		t.Fatalf("normal err = %#v, want ErrVisualforceLimitExceeded", normalErr)
	}
	if want := "visualforce iteration items limit exceeded: 1001 items > 1000 items"; normalErr.Error() != want {
		t.Fatalf("normal err = %q, want %q", normalErr.Error(), want)
	}

	if err := CheckVisualforceIterationItems(MaxVisualforceReadOnlyIterationItems, true); err != nil {
		t.Fatalf("read-only boundary err = %v", err)
	}
	readOnlyErr := CheckVisualforceIterationItems(MaxVisualforceReadOnlyIterationItems+1, true)
	if !errors.Is(readOnlyErr, ErrVisualforceLimitExceeded) {
		t.Fatalf("read-only err = %#v, want ErrVisualforceLimitExceeded", readOnlyErr)
	}
	if want := "visualforce iteration items limit exceeded: 10001 items > 10000 items"; readOnlyErr.Error() != want {
		t.Fatalf("read-only err = %q, want %q", readOnlyErr.Error(), want)
	}
}
