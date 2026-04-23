package goldsky

import (
	"context"
	"sync"
	"time"

	"timelocker-backend/internal/service/email"
	"timelocker-backend/internal/service/notification"
	"timelocker-backend/pkg/logger"
)

// flowNotificationJob 单个 flow 状态变化的通知任务
type flowNotificationJob struct {
	Standard         string
	ChainID          int
	ContractAddress  string
	FlowID           string
	StatusFrom       string
	StatusTo         string
	TxHash           *string
	InitiatorAddress string
	Source           string // 日志用：status_check / webhook
}

// NotificationDispatcher 用固定数量的 worker 消费通知队列，避免瞬时大量 goroutine
// 打爆外部 SMTP / Webhook。
type NotificationDispatcher struct {
	emailSvc        email.EmailService
	notificationSvc notification.NotificationService
	jobs            chan flowNotificationJob
	workers         int
	wg              sync.WaitGroup
	startOnce       sync.Once
	stopOnce        sync.Once
	stopped         chan struct{}
}

// NewNotificationDispatcher 创建一个通知分发器
// workers <= 0 时兜底为 4；buffer <= 0 时兜底为 1024
func NewNotificationDispatcher(
	emailSvc email.EmailService,
	notificationSvc notification.NotificationService,
	workers int,
	buffer int,
) *NotificationDispatcher {
	if workers <= 0 {
		workers = 4
	}
	if buffer <= 0 {
		buffer = 1024
	}
	return &NotificationDispatcher{
		emailSvc:        emailSvc,
		notificationSvc: notificationSvc,
		jobs:            make(chan flowNotificationJob, buffer),
		workers:         workers,
		stopped:         make(chan struct{}),
	}
}

// Start 启动 worker 池
func (d *NotificationDispatcher) Start(ctx context.Context) {
	d.startOnce.Do(func() {
		for i := 0; i < d.workers; i++ {
			d.wg.Add(1)
			go d.run(ctx, i)
		}
		logger.Info("NotificationDispatcher started", "workers", d.workers, "buffer", cap(d.jobs))
	})
}

// Stop 优雅关闭 worker 池（等待队列里剩余任务处理完）
func (d *NotificationDispatcher) Stop() {
	d.stopOnce.Do(func() {
		close(d.jobs)
		d.wg.Wait()
		close(d.stopped)
		logger.Info("NotificationDispatcher stopped")
	})
}

// Enqueue 尝试入队一条通知任务。队列满时会起临时 goroutine 兜底，确保不阻塞调用方。
func (d *NotificationDispatcher) Enqueue(job flowNotificationJob) {
	select {
	case d.jobs <- job:
	default:
		logger.Warn("NotificationDispatcher queue full, spawning overflow goroutine",
			"flow_id", job.FlowID,
			"status_to", job.StatusTo,
			"source", job.Source,
		)
		go d.process(context.Background(), job)
	}
}

func (d *NotificationDispatcher) run(ctx context.Context, idx int) {
	defer d.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-d.jobs:
			if !ok {
				return
			}
			d.process(ctx, job)
		}
	}
}

func (d *NotificationDispatcher) process(parent context.Context, job flowNotificationJob) {
	// 每个任务给 60s 超时，避免单个外部 HTTP 卡死 worker
	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()

	start := time.Now()
	logger.Info("Dispatching flow notification",
		"source", job.Source,
		"chain_id", job.ChainID,
		"contract", job.ContractAddress,
		"flow_id", job.FlowID,
		"from", job.StatusFrom,
		"to", job.StatusTo,
	)

	if d.emailSvc != nil {
		if err := d.emailSvc.SendFlowNotification(ctx, job.Standard, job.ChainID, job.ContractAddress, job.FlowID, job.StatusFrom, job.StatusTo, job.TxHash, job.InitiatorAddress); err != nil {
			logger.Error("Failed to send email notification", err,
				"chain_id", job.ChainID, "flow_id", job.FlowID, "status_to", job.StatusTo)
		}
	}

	if d.notificationSvc != nil {
		if err := d.notificationSvc.SendFlowNotification(ctx, job.Standard, job.ChainID, job.ContractAddress, job.FlowID, job.StatusFrom, job.StatusTo, job.TxHash, job.InitiatorAddress); err != nil {
			logger.Error("Failed to send channel notification", err,
				"chain_id", job.ChainID, "flow_id", job.FlowID, "status_to", job.StatusTo)
		}
	}

	logger.Info("Flow notification dispatched",
		"source", job.Source,
		"flow_id", job.FlowID,
		"status_to", job.StatusTo,
		"elapsed_ms", time.Since(start).Milliseconds(),
	)
}
