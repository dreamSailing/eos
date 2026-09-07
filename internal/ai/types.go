package ai

// Copyright (c) 2026 EOSAIOS
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import "strings"

type Capability string

const (
	CapabilityVision          Capability = "vision"
	CapabilityImageGeneration Capability = "image_generation"
	CapabilityVideoGeneration Capability = "video_generation"
	CapabilitySpeechSynthesis Capability = "speech_synthesis"
)

func (c Capability) String() string {
	return string(c)
}

func ParseCapability(s string) Capability {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case string(CapabilityVision):
		return CapabilityVision
	case string(CapabilityImageGeneration):
		return CapabilityImageGeneration
	case string(CapabilityVideoGeneration):
		return CapabilityVideoGeneration
	case string(CapabilitySpeechSynthesis):
		return CapabilitySpeechSynthesis
	default:
		return Capability(strings.ToLower(strings.TrimSpace(s)))
	}
}

// Message 表示一条对话消息
type Message struct {
	Role       string   `json:"role"`
	Content    string   `json:"content"`
	ImagePaths []string `json:"image_paths,omitempty"`
	IsMeta     bool     `json:"isMeta,omitempty"` // IsMeta 标识该消息是否为隐藏消息（用于 Skills 系统）
}
