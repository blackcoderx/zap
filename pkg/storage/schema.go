package storage

// Request represents a saved API request in YAML format
type Request struct {
	Name    string            `yaml:"name"`
	Method  string            `yaml:"method"`
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers,omitempty"`
	Query   map[string]string `yaml:"query,omitempty"`
	Body    interface{}       `yaml:"body,omitempty"`
}

// Environment represents a set of environment variables
type Environment struct {
	Name      string            `yaml:"name"`
	Variables map[string]string `yaml:",inline"`
}

// Collection represents a folder of related requests
type Collection struct {
	Name        string    `yaml:"name"`
	Description string    `yaml:"description,omitempty"`
	Requests    []Request `yaml:"requests,omitempty"`
}
