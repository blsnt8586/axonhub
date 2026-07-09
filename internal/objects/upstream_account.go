package objects

// UpstreamAccountCredentials stores sensitive upstream account credential data.
// It is intentionally generic because providers encode OAuth/API-key/custom
// credentials differently. Never return this structure to browser clients.
type UpstreamAccountCredentials struct {
	APIKey  string        `json:"apiKey,omitempty"`
	OAuth   string        `json:"oauth,omitempty"`
	RawJSON string        `json:"rawJson,omitempty"`
	Headers []HeaderEntry `json:"headers,omitempty"`
}

func (c UpstreamAccountCredentials) HasValue() bool {
	return c.APIKey != "" || c.OAuth != "" || c.RawJSON != "" || len(c.Headers) > 0
}
