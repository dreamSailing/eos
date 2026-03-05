package bridge

import "github.com/dreamSailing/vb-coding/internal/session"

func (rc *RuntimeCore) GetCompressionStats() session.CompressionStats {
	if rc == nil || rc.cm == nil {
		return session.CompressionStats{}
	}
	return rc.cm.GetCompressionStats()
}
