// Package os provides the ServiceOS abstraction layer.
package os

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// schedulerAPIImpl implements SchedulerAPI.
type schedulerAPIImpl struct {
	ctx       *ServiceContext
	serviceID string
	mu        sync.RWMutex
	tasks     map[string]*ScheduledTask
	nextID    int
}

func newSchedulerAPI(ctx *ServiceContext, serviceID string) *schedulerAPIImpl {
	return &schedulerAPIImpl{
		ctx:       ctx,
		serviceID: serviceID,
		tasks:     make(map[string]*ScheduledTask),
	}
}

func (s *schedulerAPIImpl) Schedule(ctx context.Context, task *ScheduledTask) (string, error) {
	if err := s.ctx.RequireCapability(CapScheduler); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	task.ID = fmt.Sprintf("%s-task-%d", s.serviceID, s.nextID)
	task.Status = TaskStatusPending
	s.tasks[task.ID] = task

	return task.ID, nil
}

func (s *schedulerAPIImpl) ScheduleCron(ctx context.Context, cronExpr string, task *ScheduledTask) (string, error) {
	if err := s.ctx.RequireCapability(CapScheduler); err != nil {
		return "", err
	}
	task.CronExpr = cronExpr
	return s.Schedule(ctx, task)
}

func (s *schedulerAPIImpl) ScheduleInterval(ctx context.Context, interval time.Duration, task *ScheduledTask) (string, error) {
	if err := s.ctx.RequireCapability(CapScheduler); err != nil {
		return "", err
	}
	task.Interval = interval
	return s.Schedule(ctx, task)
}

func (s *schedulerAPIImpl) Cancel(ctx context.Context, taskID string) error {
	if err := s.ctx.RequireCapability(CapScheduler); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return NewOSError("NOT_FOUND", "task not found: "+taskID)
	}
	task.Status = TaskStatusCancelled
	return nil
}

func (s *schedulerAPIImpl) List(ctx context.Context) ([]*ScheduledTask, error) {
	if err := s.ctx.RequireCapability(CapScheduler); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*ScheduledTask, 0, len(s.tasks))
	for _, t := range s.tasks {
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (s *schedulerAPIImpl) Get(ctx context.Context, taskID string) (*ScheduledTask, error) {
	if err := s.ctx.RequireCapability(CapScheduler); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return nil, NewOSError("NOT_FOUND", "task not found: "+taskID)
	}
	return task, nil
}
