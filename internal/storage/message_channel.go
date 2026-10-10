package storage

// MessageChannelMetadata records a loaded LightningMessageChannel definition.
// Like labels and static resources, channel definitions are immutable at runtime.
type MessageChannelMetadata struct {
	Name        string                `json:"name"`
	Namespace   string                `json:"namespace,omitempty"`
	MasterLabel string                `json:"masterLabel"`
	IsExposed   bool                  `json:"isExposed,omitempty"`
	Description string                `json:"description,omitempty"`
	Fields      []MessageChannelField `json:"fields,omitempty"`
	File        string                `json:"file,omitempty"`
}

type MessageChannelField struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}
