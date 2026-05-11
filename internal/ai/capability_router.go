package ai

import (
	"errors"
	"strings"

	"github.com/dreamSailing/eos/internal/config"
)

var ErrCapabilityModelUnavailable = errors.New("capability model unavailable")

type CapabilityRoute struct {
	Capability Capability
	Entry      config.ModelEntry
	Source     string
}

func ResolveCapabilityRoute(cfg config.Config, capability Capability) (CapabilityRoute, error) {
	capability = ParseCapability(capability.String())
	if active, ok := config.ActiveModel(cfg); ok && supportsCapabilityWithConfig(active, capability) {
		return CapabilityRoute{
			Capability: capability,
			Entry:      active,
			Source:     "primary",
		}, nil
	}
	if entry, ok := config.ResolveCapabilityModel(cfg, capability.String()); ok && supportsCapabilityWithConfig(entry, capability) {
		return CapabilityRoute{
			Capability: capability,
			Entry:      entry,
			Source:     "capability_model",
		}, nil
	}
	return CapabilityRoute{}, ErrCapabilityModelUnavailable
}

func supportsCapabilityWithConfig(entry config.ModelEntry, capability Capability) bool {
	if !config.ModelEnabled(entry) {
		return false
	}
	if config.SupportsCapability(entry, capability.String()) {
		return true
	}
	modelName := strings.TrimSpace(entry.Model)
	if modelName == "" {
		return false
	}
	return SupportsCapability(modelName, capability)
}
