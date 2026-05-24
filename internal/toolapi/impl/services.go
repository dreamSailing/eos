package impl

// Copyright (c) 2026 DreamSailing
// SPDX-License-Identifier: EOS-NCL-1.1
// 本文件基于 EOS 非商用许可证 v1.1 发布，详见 LICENSE。
// 商业使用请联系版权人获得商业授权。

import "github.com/dreamSailing/eos/internal/toolapi"

type services struct {
	catalog        toolapi.Catalog
	tasks          toolapi.Tasks
	newToolBackend func(workspaceRoot string) ToolExecutorBridge
}

func NewServices() toolapi.Services {
	return &services{
		catalog:        newCatalog(),
		tasks:          newTasks(newLegacyTaskSource()),
		newToolBackend: newLegacyToolExecutor,
	}
}

func (s *services) NewExecutor(workspaceRoot string) toolapi.Executor {
	return newExecutor(s.newToolBackend(workspaceRoot))
}

func (s *services) Catalog() toolapi.Catalog {
	return s.catalog
}

func (s *services) Tasks() toolapi.Tasks {
	return s.tasks
}
