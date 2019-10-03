package pkg

// ConfigEntry represents topic configuration
type ConfigEntry struct {
	Name      string
	Value     string
	ReadOnly  bool
	Default   bool
	Sensitive bool
}

// Topic represents kafka topic
type Topic struct {
	Name     string         `json:"name,omitempty"`
	Configs  []*ConfigEntry `json:"configs,omitempty"`
	ErrorMsg *string        `json:"errorMsg,omitempty"`
}

// Metadata represents topics details
type Metadata struct {
	Topics []*Topic
}
