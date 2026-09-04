package webbridge

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1

import (
	"errors"
	"strings"
)

func (svc *CapabilityService) UpsertMCP(name, kind, target string, enabled bool) (BootstrapState, error) {
	s := svc.bridge
	if s == nil {
		return BootstrapState{}, errors.New("bridge service is not available")
	}
	if err := s.upsertMCPRPC(name, kind, target, enabled); err != nil {
		return s.LoadBootstrap(), err
	}
	s.stateMu.Lock()
	s.pushNotificationLocked("MCP Saved", strings.TrimSpace(name), "success")
	s.emitShellUpdated()
	s.stateMu.Unlock()
	return s.LoadBootstrap(), nil
}

func (svc *CapabilityService) ImportMCPJSON(raw string) (BootstrapState, error) {
	s := svc.bridge
	if s == nil {
		return BootstrapState{}, errors.New("bridge service is not available")
	}
	if err := s.importMCPJSONRPC(raw); err != nil {
		return s.LoadBootstrap(), err
	}
	s.stateMu.Lock()
	s.pushNotificationLocked("MCP JSON Imported", "A new MCP configuration was saved", "success")
	s.emitShellUpdated()
	s.stateMu.Unlock()
	return s.LoadBootstrap(), nil
}

func (svc *CapabilityService) DeleteMCP(name string) (BootstrapState, error) {
	s := svc.bridge
	if s == nil {
		return BootstrapState{}, errors.New("bridge service is not available")
	}
	if err := s.deleteMCPRPC(name); err != nil {
		return s.LoadBootstrap(), err
	}
	s.stateMu.Lock()
	s.pushNotificationLocked("MCP Deleted", strings.TrimSpace(name), "warning")
	s.emitShellUpdated()
	s.stateMu.Unlock()
	return s.LoadBootstrap(), nil
}

func (svc *CapabilityService) SetMCPEnabled(name string, enabled bool) (BootstrapState, error) {
	s := svc.bridge
	if s == nil {
		return BootstrapState{}, errors.New("bridge service is not available")
	}
	if err := s.setMCPEnabledRPC(name, enabled); err != nil {
		return s.LoadBootstrap(), err
	}
	s.stateMu.Lock()
	status := "disabled"
	tone := "warning"
	if enabled {
		status = "enabled"
		tone = "success"
	}
	s.pushNotificationLocked("MCP Status Updated", strings.TrimSpace(name)+" "+status, tone)
	s.emitShellUpdated()
	s.stateMu.Unlock()
	return s.LoadBootstrap(), nil
}

func (svc *CapabilityService) DetectLSP(language string) BootstrapState {
	s := svc.bridge
	if s == nil {
		return BootstrapState{}
	}
	message, err := s.detectLSPRPC(language)
	language = strings.TrimSpace(language)
	s.stateMu.Lock()
	if err != nil {
		s.pushNotificationLocked("LSP 检测失败", fallbackText(err.Error(), language), "warning")
	} else {
		s.pushNotificationLocked("LSP Detection Complete", fallbackText(message, language), "info")
	}
	s.emitShellUpdated()
	s.stateMu.Unlock()
	return s.LoadBootstrap()
}

func (svc *CapabilityService) StartLSP(language string) BootstrapState {
	s := svc.bridge
	if s == nil {
		return BootstrapState{}
	}
	message, err := s.startLSPRPC(language)
	language = strings.TrimSpace(language)
	s.stateMu.Lock()
	if err != nil {
		s.pushNotificationLocked("LSP 启动失败", fallbackText(err.Error(), language), "warning")
	} else {
		s.pushNotificationLocked("LSP Started", fallbackText(message, language), "success")
	}
	s.emitShellUpdated()
	s.stateMu.Unlock()
	return s.LoadBootstrap()
}

func (svc *CapabilityService) ReloadSkills() (BootstrapState, error) {
	s := svc.bridge
	if s == nil {
		return BootstrapState{}, errors.New("bridge service is not available")
	}
	if err := s.reloadSkillsRPC(); err != nil {
		return s.LoadBootstrap(), err
	}
	s.stateMu.Lock()
	s.pushNotificationLocked("Skills Reloaded", "The skill catalog was refreshed", "success")
	s.emitShellUpdated()
	s.stateMu.Unlock()
	return s.LoadBootstrap(), nil
}

func (svc *CapabilityService) ReloadSkillsSilent() (BootstrapState, error) {
	s := svc.bridge
	if s == nil {
		return BootstrapState{}, errors.New("bridge service is not available")
	}
	if err := s.reloadSkillsRPC(); err != nil {
		return s.LoadBootstrap(), err
	}
	s.stateMu.Lock()
	s.emitShellUpdated()
	s.stateMu.Unlock()
	return s.LoadBootstrap(), nil
}

func (svc *CapabilityService) SetSkillEnabled(name string, enabled bool) (BootstrapState, error) {
	s := svc.bridge
	if s == nil {
		return BootstrapState{}, errors.New("bridge service is not available")
	}
	if err := s.setSkillEnabledRPC(name, enabled); err != nil {
		return s.LoadBootstrap(), err
	}
	s.stateMu.Lock()
	status := "disabled"
	tone := "warning"
	if enabled {
		status = "enabled"
		tone = "success"
	}
	s.pushNotificationLocked("Skill Status Updated", strings.TrimSpace(name)+" "+status, tone)
	s.emitShellUpdated()
	s.stateMu.Unlock()
	return s.LoadBootstrap(), nil
}

func (svc *CapabilityService) SetPluginEnabled(name string, enabled bool) (BootstrapState, error) {
	s := svc.bridge
	if s == nil {
		return BootstrapState{}, errors.New("bridge service is not available")
	}
	if err := s.setPluginEnabledRPC(name, enabled); err != nil {
		return s.LoadBootstrap(), err
	}
	s.stateMu.Lock()
	status := "disabled"
	tone := "warning"
	if enabled {
		status = "enabled"
		tone = "success"
	}
	s.pushNotificationLocked("Plugin Status Updated", strings.TrimSpace(name)+" "+status, tone)
	s.emitShellUpdated()
	s.stateMu.Unlock()
	return s.LoadBootstrap(), nil
}
