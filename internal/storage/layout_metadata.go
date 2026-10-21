package storage

// LayoutMetadata retains a deployed layout declaration independently of schema
// fields. Its XML is immutable across runtime org clones.
type LayoutMetadata struct {
	FullName string `json:"fullName"`
	XML      string `json:"xml"`
}
