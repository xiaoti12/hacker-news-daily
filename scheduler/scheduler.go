package scheduler

import (
	"log"

	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron *cron.Cron
}

type JobFunc func() error

func NewScheduler() *Scheduler {
	c := cron.New(cron.WithSeconds())
	return &Scheduler{cron: c}
}

// AddJob 添加定时任务
func (s *Scheduler) AddJob(cronExpr string, job JobFunc) error {
	_, err := s.cron.AddFunc(cronExpr, func() {
		if err := job(); err != nil {
			log.Printf("Job execution failed: %v", err)
		}
	})
	return err
}

// Start 启动调度器
func (s *Scheduler) Start() {
	s.cron.Start()
	log.Println("Scheduler started")
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.cron.Stop()
	log.Println("Scheduler stopped")
}

// RunOnce 立即执行一次任务（用于测试）
func (s *Scheduler) RunOnce(job JobFunc) error {
	log.Println("Running job once...")
	return job()
}
