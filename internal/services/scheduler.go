package services

import (
	"log"
	"sync"
	"time"
)

// schedulerService 实现 SchedulerService 接口
type schedulerService struct {
	jobs    map[string]*ScheduledJob
	mu      sync.RWMutex
	started bool
	done    chan struct{}
	wg      sync.WaitGroup
}

// ScheduledJob 定时任务结构
type ScheduledJob struct {
	Name     string
	Interval time.Duration
	Job      func()
	ticker   *time.Ticker
	done     chan struct{}
}

// NewSchedulerService 创建新的调度服务
func NewSchedulerService() SchedulerService {
	return &schedulerService{
		jobs: make(map[string]*ScheduledJob),
		done: make(chan struct{}),
	}
}

// AddJob 添加定时任务
func (s *schedulerService) AddJob(name string, interval time.Duration, job func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// 如果任务已存在，先停止它
	if existingJob, exists := s.jobs[name]; exists {
		s.stopJob(existingJob)
	}
	
	scheduledJob := &ScheduledJob{
		Name:     name,
		Interval: interval,
		Job:      job,
		done:     make(chan struct{}),
	}
	
	s.jobs[name] = scheduledJob
	
	// 如果调度器已启动，立即启动这个任务
	if s.started {
		s.startJob(scheduledJob)
	}
	
	log.Printf("Added scheduled job: %s (interval: %v)", name, interval)
}

// RemoveJob 移除定时任务
func (s *schedulerService) RemoveJob(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if job, exists := s.jobs[name]; exists {
		s.stopJob(job)
		delete(s.jobs, name)
		log.Printf("Removed scheduled job: %s", name)
	}
}

// Start 启动调度器
func (s *schedulerService) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.started {
		log.Println("Scheduler already started")
		return
	}
	
	s.started = true
	log.Printf("Starting scheduler with %d jobs", len(s.jobs))
	
	// 启动所有任务
	for _, job := range s.jobs {
		s.startJob(job)
	}
}

// Stop 停止调度器
func (s *schedulerService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.started {
		return
	}
	
	log.Println("Stopping scheduler...")
	
	// 停止所有任务
	for _, job := range s.jobs {
		s.stopJob(job)
	}
	
	// 发送停止信号
	close(s.done)
	s.started = false
	
	// 等待所有goroutine完成
	s.wg.Wait()
	log.Println("Scheduler stopped")
}

// startJob 启动单个任务
func (s *schedulerService) startJob(job *ScheduledJob) {
	job.ticker = time.NewTicker(job.Interval)
	
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer job.ticker.Stop()
		
		log.Printf("Started job: %s", job.Name)
		
		for {
			select {
			case <-job.ticker.C:
				// 执行任务
				go func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("Job %s panicked: %v", job.Name, r)
						}
					}()
					job.Job()
				}()
			case <-job.done:
				log.Printf("Job %s stopped", job.Name)
				return
			case <-s.done:
				log.Printf("Job %s stopped (scheduler shutdown)", job.Name)
				return
			}
		}
	}()
}

// stopJob 停止单个任务
func (s *schedulerService) stopJob(job *ScheduledJob) {
	if job.ticker != nil {
		job.ticker.Stop()
		job.ticker = nil
	}
	
	select {
	case <-job.done:
		// 已经关闭
	default:
		close(job.done)
	}
}

// GetJobCount 获取任务数量
func (s *schedulerService) GetJobCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.jobs)
}

// GetJobNames 获取所有任务名称
func (s *schedulerService) GetJobNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	names := make([]string, 0, len(s.jobs))
	for name := range s.jobs {
		names = append(names, name)
	}
	return names
}