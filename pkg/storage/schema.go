package storage

// Request represents a saved API request in YAML format.
type Request struct {
	Name    string            `yaml:"name"`              // Unique name for the request
	Method  string            `yaml:"method"`            // HTTP method (GET, POST, etc.)
	URL     string            `yaml:"url"`               // Request URL (can contain variables)
	Headers map[string]string `yaml:"headers,omitempty"` // HTTP headers
	Query   map[string]string `yaml:"query,omitempty"`   // Query parameters
	Body    interface{}       `yaml:"body,omitempty"`    // Request body (JSON or string)
}

// Environment represents a set of environment variables.
type Environment struct {
	Name      string            `yaml:"name"`    // Environment name (e.g., "dev", "prod")
	Variables map[string]string `yaml:",inline"` // Key-value pairs for variables
}

// Collection represents a folder of related requests.
type Collection struct {
	Name        string    `yaml:"name"`                  // Collection name
	Description string    `yaml:"description,omitempty"` // Optional description
	Requests    []Request `yaml:"requests,omitempty"`    // List of requests in the collection
}
