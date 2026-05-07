package scheduler

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dreamSailing/eos/internal/toolapi"
	"github.com/google/uuid"
)

type Service struct {
	mu               sync.RWMutex
	store            *Store
	services         toolapi.Services
	defaultWorkspace string
	items            map[string]*Schedule
	closeCh          chan struct{}
	once             sync.Once
}

func NewService(storePath string, services toolapi.Services, defaultWorkspace string) *Service {
	return &Service{
		store:            NewStore(storePath),
		services:         services,
		defaultWorkspace: strings.TrimSpace(defaultWorkspace),
		items:            map[string]*Schedule{},
		closeCh:          make(chan struct{}),
	}
}

func (s *Service) Start(ctx context.Context) error {
	if err := s.reload(); err != nil {
		return err
	}
	go s.loop(ctx)
	return nil
}

func (s *Service) Close() {
	s.once.Do(func() { close(s.closeCh) })
}

func (s *Service) loop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.closeCh:
			return
		case <-ticker.C:
			s.runDue(context.Background())
		}
	}
}

func (s *Service) reload() error {
	items, err := s.store.Load()
	if err != nil {
		return err
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = map[string]*Schedule{}
	for _, item := range items {
		it := item
		if strings.TrimSpace(it.Workspace) == "" {
			it.Workspace = s.defaultWorkspace
		}
		if it.Enabled {
			it.NextRunAt, _ = nextRun(it.Cron, now)
		}
		s.items[it.ID] = &it
	}
	return nil
}

func (s *Service) persistLocked() error {
	items := make([]Schedule, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, cloneSchedule(item))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return s.store.Save(items)
}

func (s *Service) List() []Schedule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]Schedule, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, cloneSchedule(item))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].ID < items[j].ID
		}
		return items[i].Name < items[j].Name
	})
	return items
}

func (s *Service) Get(id string) (Schedule, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item := s.items[strings.TrimSpace(id)]
	if item == nil {
		return Schedule{}, false
	}
	return cloneSchedule(item), true
}

func (s *Service) Create(in Schedule) (Schedule, error) {
	now := time.Now()
	item, err := s.normalize(in, true, now)
	if err != nil {
		return Schedule{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if item.ID == "" {
		item.ID = "sch_" + uuid.NewString()[:12]
	}
	s.items[item.ID] = &item
	if err := s.persistLocked(); err != nil {
		return Schedule{}, err
	}
	return cloneSchedule(&item), nil
}

func (s *Service) Update(id string, in Schedule) (Schedule, error) {
	now := time.Now()
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	current := s.items[id]
	if current == nil {
		return Schedule{}, fmt.Errorf("schedule not found")
	}
	in.ID = id
	item, err := s.normalize(in, false, now)
	if err != nil {
		return Schedule{}, err
	}
	item.LastRunAt = current.LastRunAt
	item.LastStatus = current.LastStatus
	item.LastError = current.LastError
	if item.Enabled {
		item.NextRunAt, _ = nextRun(item.Cron, now)
	}
	s.items[id] = &item
	if err := s.persistLocked(); err != nil {
		return Schedule{}, err
	}
	return cloneSchedule(&item), nil
}

func (s *Service) Delete(id string) error {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.items[id] == nil {
		return fmt.Errorf("schedule not found")
	}
	delete(s.items, id)
	return s.persistLocked()
}

func (s *Service) TriggerNow(ctx context.Context, id string) (Schedule, error) {
	id = strings.TrimSpace(id)
	s.mu.RLock()
	item := s.items[id]
	if item == nil {
		s.mu.RUnlock()
		return Schedule{}, fmt.Errorf("schedule not found")
	}
	copyItem := cloneSchedule(item)
	s.mu.RUnlock()
	go s.executeSchedule(ctx, copyItem)
	return copyItem, nil
}

func (s *Service) runDue(ctx context.Context) {
	now := time.Now()
	due := make([]Schedule, 0)
	s.mu.Lock()
	for _, item := range s.items {
		if item == nil || !item.Enabled || item.NextRunAt.IsZero() || item.NextRunAt.After(now) {
			continue
		}
		copyItem := cloneSchedule(item)
		item.NextRunAt, _ = nextRun(item.Cron, now)
		due = append(due, copyItem)
	}
	_ = s.persistLocked()
	s.mu.Unlock()
	for _, item := range due {
		go s.executeSchedule(ctx, item)
	}
}

func (s *Service) executeSchedule(ctx context.Context, item Schedule) {
	now := time.Now()
	status := "success"
	errText := ""
	workspace := strings.TrimSpace(item.Workspace)
	if workspace == "" {
		workspace = s.defaultWorkspace
	}
	var err error
	switch item.Kind {
	case TaskKindShell:
		err = runShellSchedule(item, workspace)
	case TaskKindEOSCall:
		err = runEOSSchedule(ctx, s.services, item, workspace)
	default:
		err = fmt.Errorf("unsupported schedule kind: %s", item.Kind)
	}
	if err != nil {
		status = "error"
		errText = err.Error()
	}
	s.mu.Lock()
	if current := s.items[item.ID]; current != nil {
		current.LastRunAt = now
		current.LastStatus = status
		current.LastError = errText
		if current.Enabled && current.NextRunAt.IsZero() {
			current.NextRunAt, _ = nextRun(current.Cron, now)
		}
		_ = s.persistLocked()
	}
	s.mu.Unlock()
}

func (s *Service) normalize(in Schedule, isCreate bool, now time.Time) (Schedule, error) {
	in.ID = strings.TrimSpace(in.ID)
	in.Name = strings.TrimSpace(in.Name)
	in.Cron = strings.TrimSpace(in.Cron)
	in.Workspace = strings.TrimSpace(in.Workspace)
	if in.Name == "" {
		return Schedule{}, fmt.Errorf("name required")
	}
	if in.Cron == "" {
		return Schedule{}, fmt.Errorf("cron required")
	}
	if _, err := nextRun(in.Cron, now); err != nil {
		return Schedule{}, err
	}
	if in.Workspace == "" {
		in.Workspace = s.defaultWorkspace
	}
	if in.Kind != TaskKindEOSCall && in.Kind != TaskKindShell {
		return Schedule{}, fmt.Errorf("kind required")
	}
	if in.Payload == nil {
		in.Payload = map[string]any{}
	}
	if isCreate && in.ID == "" {
		in.ID = "sch_" + uuid.NewString()[:12]
	}
	if in.Enabled {
		in.NextRunAt, _ = nextRun(in.Cron, now)
	}
	return in, nil
}

func cloneSchedule(in *Schedule) Schedule {
	if in == nil {
		return Schedule{}
	}
	out := *in
	if in.Payload != nil {
		out.Payload = make(map[string]any, len(in.Payload))
		for k, v := range in.Payload {
			out.Payload[k] = v
		}
	}
	return out
}
