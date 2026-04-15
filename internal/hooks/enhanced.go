package hooks

// HTTPHandler represents an HTTP-based hook handler
type HTTPHandler struct {
	URL            string            `json:"url" yaml:"url"`
	Method         string            `json:"method" yaml:"method"`
	Headers        map[string]string `json:"headers" yaml:"headers"`
	Timeout        int               `json:"timeout" yaml:"timeout"`
	ExpectedStatus int               `json:"expectedStatus" yaml:"expectedStatus"`
}

// EnhancedHandler extends Handler with HTTP and glob matcher support
type EnhancedHandler struct {
	Handler
	HTTP   *HTTPHandler `json:"http,omitempty" yaml:"http,omitempty"`
	Globber string       `json:"globMatcher,omitempty" yaml:"globMatcher,omitempty"`
}

// MatcherType returns the type of matcher used
func (h *EnhancedHandler) MatcherType() string {
	if h.HTTP != nil {
		return "http"
	}
	if h.Globber != "" {
		return "glob"
	}
	return h.Type
}
