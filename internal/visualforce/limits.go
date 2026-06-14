package visualforce

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	MaxVisualforceResponseBytes                = 15 * 1000 * 1000
	MaxVisualforceViewStateBytes               = 170 * 1024
	MaxVisualforceEmailTemplateBytes           = 1 * 1000 * 1000
	MaxVisualforceUploadBytes                  = 10 * 1000 * 1000
	MaxVisualforcePDFHTMLResponseBytes         = 15 * 1000 * 1000
	MaxVisualforcePDFBytes                     = 60 * 1000 * 1000
	MaxVisualforcePDFImageBytes                = 30 * 1000 * 1000
	MaxVisualforceHeaderBytes                  = 8192
	MaxVisualforceRemotingRequestBytes         = 4 * 1000 * 1000
	DefaultVisualforceRemotingTimeout          = 30 * time.Second
	MaxVisualforceRemotingTimeout              = 120 * time.Second
	MaxVisualforceQueryRows                    = 50000
	MaxVisualforceReadOnlyQueryRows            = 1000000
	MaxVisualforceIterationItems               = 1000
	MaxVisualforceReadOnlyIterationItems       = 10000
	MaxVisualforceStandardSetControllerRecords = 10000
	MaxVisualforcePageFieldSets                = 50
	MaxVisualforceSObjectFieldSets             = 2000
	MaxVisualforceFieldSetLookupFields         = 25
)

var ErrVisualforceLimitExceeded = errors.New("visualforce limit exceeded")

type VisualforceLimitError struct {
	Name      string
	Actual    int
	Max       int
	Inclusive bool
	Unit      string
}

func (e *VisualforceLimitError) Error() string {
	if e == nil {
		return ""
	}
	operator := ">"
	if e.Inclusive {
		operator = ">="
	}
	unit := strings.TrimSpace(e.Unit)
	if unit == "" {
		unit = "bytes"
	}
	return fmt.Sprintf("visualforce %s limit exceeded: %d %s %s %d %s", e.Name, e.Actual, unit, operator, e.Max, unit)
}

func (e *VisualforceLimitError) Unwrap() error {
	return ErrVisualforceLimitExceeded
}

type VisualforceLimits struct {
	QueryRows      int
	IterationItems int
}

func VisualforceRequestLimits(readOnly bool) VisualforceLimits {
	if readOnly {
		return VisualforceLimits{
			QueryRows:      MaxVisualforceReadOnlyQueryRows,
			IterationItems: MaxVisualforceReadOnlyIterationItems,
		}
	}
	return VisualforceLimits{
		QueryRows:      MaxVisualforceQueryRows,
		IterationItems: MaxVisualforceIterationItems,
	}
}

func CheckVisualforceViewStateSize(size int) error {
	return checkVisualforceByteLimit("view state", size, MaxVisualforceViewStateBytes, false)
}

func CheckVisualforceUploadSize(size int) error {
	return checkVisualforceByteLimit("upload", size, MaxVisualforceUploadBytes, false)
}

func CheckVisualforceRemotingRequestSize(size int) error {
	return checkVisualforceByteLimit("remoting request", size, MaxVisualforceRemotingRequestBytes, false)
}

func CheckVisualforceHeaderSize(size int) error {
	return checkVisualforceByteLimit("header", size, MaxVisualforceHeaderBytes, false)
}

func CheckVisualforceResponseSize(size int) error {
	return checkVisualforceByteLimit("response", size, MaxVisualforceResponseBytes, true)
}

func CheckVisualforcePDFHTMLResponseSize(size int) error {
	return checkVisualforceByteLimit("pdf html response", size, MaxVisualforcePDFHTMLResponseBytes, true)
}

func CheckVisualforcePDFSize(size int) error {
	return checkVisualforceByteLimit("pdf", size, MaxVisualforcePDFBytes, false)
}

func CheckVisualforcePDFImageSize(size int) error {
	return checkVisualforceByteLimit("pdf image", size, MaxVisualforcePDFImageBytes, false)
}

func CheckVisualforceIterationItems(count int, readOnly bool) error {
	limit := MaxVisualforceIterationItems
	if readOnly {
		limit = MaxVisualforceReadOnlyIterationItems
	}
	return checkVisualforceCountLimit("iteration items", count, limit)
}

func checkVisualforceByteLimit(name string, actual, max int, inclusive bool) error {
	if inclusive {
		if actual >= max {
			return &VisualforceLimitError{Name: name, Actual: actual, Max: max, Inclusive: true}
		}
		return nil
	}
	if actual > max {
		return &VisualforceLimitError{Name: name, Actual: actual, Max: max}
	}
	return nil
}

func checkVisualforceCountLimit(name string, actual, max int) error {
	if actual > max {
		return &VisualforceLimitError{Name: name, Actual: actual, Max: max, Unit: "items"}
	}
	return nil
}
