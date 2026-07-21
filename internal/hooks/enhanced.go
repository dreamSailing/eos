package hooks

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

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
	HTTP    *HTTPHandler `json:"http,omitempty" yaml:"http,omitempty"`
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
