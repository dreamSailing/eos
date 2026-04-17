package impl

import "github.com/dreamSailing/eos/internal/toolapi"

type services struct {
	catalog toolapi.Catalog
	tasks   toolapi.Tasks
}

func NewServices() toolapi.Services {
	return &services{
		catalog: newCatalog(),
		tasks:   newTasks(),
	}
}

func (s *services) NewExecutor(workspaceRoot string) toolapi.Executor {
	return newExecutor(workspaceRoot)
}

func (s *services) Catalog() toolapi.Catalog {
	return s.catalog
}

func (s *services) Tasks() toolapi.Tasks {
	return s.tasks
}

