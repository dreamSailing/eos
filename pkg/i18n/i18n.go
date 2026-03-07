package i18n

import internal "github.com/dreamSailing/vb-coding/internal/i18n"

func T(key string, lang string, args ...any) string {
	return internal.T(key, lang, args...)
}
