package project

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strings"
)

// StandardSharingReasonRejection names an object file whose sharing reason
// Salesforce rejects at deploy time.
type StandardSharingReasonRejection struct {
	File    string
	Message string
}

var sharingReasonObjectSuffixes = []string{"__as", "__b", "__c", "__e", "__kav", "__mdt", "__x"}

// accountSharingReasonFiles returns the Account object files that declare
// sharing reasons inline. The API 68 capture (apextest api68_native
// sharing-off SHARE-REASON-DEPLOY) covers only the Account object file in a
// Developer scratch org; other standard objects and the decomposed
// sharingReasons directory were not captured.
func accountSharingReasonFiles(p Project, read func(string) ([]byte, error)) map[string][]string {
	var out map[string][]string
	for _, file := range p.ObjectFiles {
		base := filepath.Base(file)
		if !strings.EqualFold(base, "Account.object-meta.xml") && !strings.EqualFold(base, "Account.object") {
			continue
		}
		data, err := read(file)
		if err != nil || !bytes.Contains(data, []byte("sharingReasons")) {
			continue
		}
		var object struct {
			Reasons []struct {
				FullName string `xml:"fullName"`
			} `xml:"sharingReasons"`
		}
		if err := xml.Unmarshal(data, &object); err != nil {
			continue
		}
		for _, reason := range object.Reasons {
			if out == nil {
				out = make(map[string][]string)
			}
			out[file] = append(out[file], strings.TrimSpace(reason.FullName))
		}
	}
	return out
}

// AccountSharingReasonShapes keys the declared Account sharing reason names by
// file so cached index results can be bound to them.
func AccountSharingReasonShapes(p Project, read func(string) ([]byte, error)) map[string]string {
	if read == nil {
		read = p.ReadFile
	}
	reasons := accountSharingReasonFiles(p, read)
	if len(reasons) == 0 {
		return nil
	}
	shapes := make(map[string]string, len(reasons))
	for file, names := range reasons {
		shapes[file] = strings.Join(names, ",")
	}
	return shapes
}

// StandardSharingReasonRejections reports inline Account sharing reasons whose
// name lacks a custom object suffix, with the native component problem.
func StandardSharingReasonRejections(p Project, read func(string) ([]byte, error)) []StandardSharingReasonRejection {
	if read == nil {
		read = p.ReadFile
	}
	reasons := accountSharingReasonFiles(p, read)
	var rejections []StandardSharingReasonRejection
	for _, file := range p.ObjectFiles {
		for _, name := range reasons[file] {
			suffixed := false
			for _, suffix := range sharingReasonObjectSuffixes {
				suffixed = suffixed || strings.HasSuffix(strings.ToLower(name), suffix)
			}
			if name == "" || suffixed {
				continue
			}
			rejections = append(rejections, StandardSharingReasonRejection{
				File:    file,
				Message: fmt.Sprintf("'%s' does not resolve to a valid sObject type, fullName must end with: %s", name, strings.Join(sharingReasonObjectSuffixes, ", ")),
			})
		}
	}
	return rejections
}
