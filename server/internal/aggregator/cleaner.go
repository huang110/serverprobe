package aggregator

import (
	"log"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/huang110/serverprobe/server/internal/model"
)

const (
	// rawRetentionDays 原始指标保留天数
	rawRetentionDays = 7

	// fiveMinuteRetentionDays 5m 聚合保留天数
	fiveMinuteRetentionDays = 30

	// oneHourRetentionDays 1h 聚合保留天数
	oneHourRetentionDays = 90

	// oneDayRetentionDays 1d 聚合保留天数
	oneDayRetentionDays = 365

	// rawScanMonths 清理原始分区时回溯的月份数
	rawScanMonths = 12
)

// Cleaner 数据清理任务
type Cleaner struct {
	db     *gorm.DB
	stopCh chan struct{}
	doneCh chan struct{}

	stopOnce sync.Once
	mu       sync.Mutex
	started  bool
}

// NewCleaner 创建清理任务
func NewCleaner(db *gorm.DB) *Cleaner {
	return &Cleaner{
		db:     db,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

// Start 启动清理任务，每天 UTC 2 点执行
func (c *Cleaner) Start() {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return
	}
	c.started = true
	c.mu.Unlock()

	defer close(c.doneCh)

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case now := <-ticker.C:
			if now.UTC().Hour() == 2 {
				if err := c.CleanOldData(now.UTC()); err != nil {
					log.Printf("data cleanup failed: %v", err)
				}
			}
		case <-c.stopCh:
			return
		}
	}
}

// Stop 停止清理任务
func (c *Cleaner) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})

	c.mu.Lock()
	started := c.started
	c.mu.Unlock()
	if started {
		<-c.doneCh
	}
}

// CleanOldData 清理过期数据
func (c *Cleaner) CleanOldData(now time.Time) error {
	if c.db == nil {
		return nil
	}

	rawThreshold := now.AddDate(0, 0, -rawRetentionDays).Unix()
	if err := c.cleanRawMetrics(rawThreshold); err != nil {
		return err
	}

	if err := c.db.Where("timestamp < ?", rawThreshold).Delete(&model.MetricsDisk{}).Error; err != nil {
		return err
	}
	if err := c.db.Where("timestamp < ?", rawThreshold).Delete(&model.MetricsNetwork{}).Error; err != nil {
		return err
	}

	if err := c.db.Where("granularity = ? AND timestamp < ?", "5m",
		now.AddDate(0, 0, -fiveMinuteRetentionDays).Unix()).
		Delete(&model.MetricsAggregated{}).Error; err != nil {
		return err
	}
	if err := c.db.Where("granularity = ? AND timestamp < ?", "1h",
		now.AddDate(0, 0, -oneHourRetentionDays).Unix()).
		Delete(&model.MetricsAggregated{}).Error; err != nil {
		return err
	}
	if err := c.db.Where("granularity = ? AND timestamp < ?", "1d",
		now.AddDate(0, 0, -oneDayRetentionDays).Unix()).
		Delete(&model.MetricsAggregated{}).Error; err != nil {
		return err
	}

	log.Printf("data cleanup completed")
	return nil
}

// cleanRawMetrics 清理原始分区表中过期的数据
func (c *Cleaner) cleanRawMetrics(threshold int64) error {
	start := time.Unix(threshold, 0).UTC().AddDate(0, -rawScanMonths, 0)
	end := time.Unix(threshold, 0).UTC()

	for cursor := beginningOfMonthUTC(start); cursor.Before(end); cursor = cursor.AddDate(0, 1, 0) {
		tableName := model.GetMetricsRawTableName(cursor)
		if !c.db.Migrator().HasTable(tableName) {
			continue
		}
		if err := c.db.Exec("DELETE FROM "+tableName+" WHERE timestamp < ?", threshold).Error; err != nil {
			return err
		}
	}
	return nil
}
