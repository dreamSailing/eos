package i18n

import "fmt"

var textByLang = map[string]map[string]string{
	"zh": zhText,
	"en": enText,
}

func T(key string, lang string, args ...any) string {
	m := textByLang[lang]
	if m == nil {
		m = textByLang["zh"]
	}
	s := m[key]
	if s == "" {
		// Fallback to English if not found in current language
		if lang != "en" {
			if enM := textByLang["en"]; enM != nil {
				s = enM[key]
			}
		}
		if s == "" {
			s = key
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}
