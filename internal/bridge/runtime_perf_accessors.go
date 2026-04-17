package bridge

import "github.com/dreamSailing/eos/internal/session"

func (rc *RuntimeCore) GetCompressionStats() session.CompressionStats {
	if rc == nil || rc.cm == nil {
		return session.CompressionStats{}
	}
	return rc.cm.GetCompressionStats()
}
