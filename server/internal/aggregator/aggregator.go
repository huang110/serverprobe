package aggregator

import (
	"errors"
	"log"
	"math"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/huang110/serverprobe/server/internal/model"
)

const (
	// aggregateInterval 聚合任务运行间隔
	aggregateInterval = 5 * time.Minute

	// fiveMinuteSeconds 5 分钟秒数
	fiveMinuteSeconds = int64(300)

	// oneHourSeconds 1 小时秒数
	oneHourSeconds = int64(3600)

	// oneDaySeconds 1 天秒数
	oneDaySeconds = int64(86400)
)

// aggregateRow 聚合结果
type aggregateRow struct {
	AgentID           string
	CPUAvg            float64
	CPUMax            float64
	MemoryAvg         float64
	MemoryMax         float64
	Load1Avg          float64
	Load1Max          float64
	NetRxBytesSum     int64
	NetTxBytesSum     int64
	TCPEstablishedAvg float64
	SampleCount       int64
}

// Aggregator 数据聚合任务
type Aggregator struct {
	db                   *gorm.DB
	interval             time.Duration
	includeCurrentBucket bool
	stopCh               chan struct{}
	doneCh               chan struct{}

	stopOnce sync.Once
	mu       sync.Mutex
	started  bool
}

// NewAggregator 创建聚合任务，默认每 5 分钟运行一次
func NewAggregator(db *gorm.DB) *Aggregator {
	return NewAggregatorWithOptions(db, aggregateInterval, false)
}

// NewAggregatorWithInterval 创建指定运行间隔的聚合任务
func NewAggregatorWithInterval(db *gorm.DB, interval time.Duration) *Aggregator {
	return NewAggregatorWithOptions(db, interval, false)
}

// NewAggregatorWithOptions 创建聚合任务，可指定是否聚合当前时间桶
func NewAggregatorWithOptions(db *gorm.DB, interval time.Duration, includeCurrentBucket bool) *Aggregator {
	if interval <= 0 {
		interval = aggregateInterval
	}
	return &Aggregator{
		db:                   db,
		interval:             interval,
		includeCurrentBucket: includeCurrentBucket,
		stopCh:               make(chan struct{}),
		doneCh:               make(chan struct{}),
	}
}

// Start 启动聚合任务，阻塞运行直到 Stop
func (a *Aggregator) Start() {
	a.mu.Lock()
	if a.started {
		a.mu.Unlock()
		return
	}
	a.started = true
	a.mu.Unlock()

	defer close(a.doneCh)

	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()

	a.aggregateAll(time.Now().UTC())

	for {
		select {
		case now := <-ticker.C:
			a.aggregateAll(now.UTC())
		case <-a.stopCh:
			return
		}
	}
}

// Stop 停止聚合任务
func (a *Aggregator) Stop() {
	a.stopOnce.Do(func() {
		close(a.stopCh)
	})

	a.mu.Lock()
	started := a.started
	a.mu.Unlock()
	if started {
		<-a.doneCh
	}
}

// aggregateAll 执行全部聚合
func (a *Aggregator) aggregateAll(now time.Time) {
	if _, err := a.Aggregate5m(now); err != nil {
		log.Printf("failed to aggregate 5m: %v", err)
	}
	if _, err := a.Aggregate1h(now); err != nil {
		log.Printf("failed to aggregate 1h: %v", err)
	}
	if _, err := a.Aggregate1d(now); err != nil {
		log.Printf("failed to aggregate 1d: %v", err)
	}
}

// Aggregate5m 聚合 1m 原始数据到 5m
func (a *Aggregator) Aggregate5m(now time.Time) (int, error) {
	if a.db == nil {
		return 0, errors.New("database is required")
	}

	windowEnd := alignTo(now.Unix(), fiveMinuteSeconds)
	windowStart := windowEnd - fiveMinuteSeconds
	if a.includeCurrentBucket {
		windowStart = windowEnd
		windowEnd = windowStart + fiveMinuteSeconds
	}

	points := make([]aggregateRow, 0)
	startTime := time.Unix(windowStart, 0).UTC()
	endTime := time.Unix(windowEnd, 0).UTC()

	for cursor := beginningOfMonthUTC(startTime); !cursor.After(endTime); cursor = cursor.AddDate(0, 1, 0) {
		tableName := model.GetMetricsRawTableName(cursor)
		if !a.db.Migrator().HasTable(tableName) {
			continue
		}

		var rows []aggregateRow
		err := a.db.Table(tableName).
			Select(`
				agent_id,
				AVG(cpu_usage) AS cpu_avg,
				MAX(cpu_usage) AS cpu_max,
				AVG(memory_percent) AS memory_avg,
				MAX(memory_percent) AS memory_max,
				AVG(load1) AS load1_avg,
				MAX(load1) AS load1_max,
				SUM(net_rx_bytes) AS net_rx_bytes_sum,
				SUM(net_tx_bytes) AS net_tx_bytes_sum,
				AVG(tcp_estab) AS tcp_established_avg,
				COUNT(*) AS sample_count
			`).
			Where("timestamp >= ? AND timestamp < ?", windowStart, windowEnd).
			Group("agent_id").
			Scan(&rows).Error
		if err != nil {
			return 0, err
		}
		points = append(points, rows...)
	}

	return len(points), a.storeAggregated("5m", windowStart, points)
}

// Aggregate1h 聚合 5m 数据到 1h
func (a *Aggregator) Aggregate1h(now time.Time) (int, error) {
	return a.aggregateFromAggregated(now, oneHourSeconds, "5m", "1h")
}

// Aggregate1d 聚合 1h 数据到 1d
func (a *Aggregator) Aggregate1d(now time.Time) (int, error) {
	return a.aggregateFromAggregated(now, oneDaySeconds, "1h", "1d")
}

// aggregateFromAggregated 从上一级聚合表聚合
func (a *Aggregator) aggregateFromAggregated(now time.Time, windowSeconds int64, sourceGranularity, targetGranularity string) (int, error) {
	if a.db == nil {
		return 0, errors.New("database is required")
	}

	windowEnd := alignTo(now.Unix(), windowSeconds)
	windowStart := windowEnd - windowSeconds

	var rows []aggregateRow
	err := a.db.Table((model.MetricsAggregated{}).TableName()).
		Select(`
			agent_id,
			AVG(cpu_avg) AS cpu_avg,
			MAX(cpu_max) AS cpu_max,
			AVG(memory_avg) AS memory_avg,
			MAX(memory_max) AS memory_max,
			AVG(load1_avg) AS load1_avg,
			MAX(load1_max) AS load1_max,
			SUM(net_rx_bytes_sum) AS net_rx_bytes_sum,
			SUM(net_tx_bytes_sum) AS net_tx_bytes_sum,
			AVG(tcp_established_avg) AS tcp_established_avg,
			SUM(sample_count) AS sample_count
		`).
		Where("granularity = ? AND timestamp >= ? AND timestamp < ?", sourceGranularity, windowStart, windowEnd).
		Group("agent_id").
		Scan(&rows).Error
	if err != nil {
		return 0, err
	}

	return len(rows), a.storeAggregated(targetGranularity, windowStart, rows)
}

// storeAggregated 删除同一窗口的旧聚合数据并写入新结果
func (a *Aggregator) storeAggregated(granularity string, timestamp int64, rows []aggregateRow) error {
	if len(rows) == 0 {
		return nil
	}

	return a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("granularity = ? AND timestamp = ?", granularity, timestamp).
			Delete(&model.MetricsAggregated{}).Error; err != nil {
			return err
		}

		records := make([]model.MetricsAggregated, 0, len(rows))
		for _, row := range rows {
			records = append(records, model.MetricsAggregated{
				AgentID:           row.AgentID,
				Granularity:       granularity,
				Timestamp:         timestamp,
				CPUAvg:            row.CPUAvg,
				CPUMax:            row.CPUMax,
				MemoryAvg:         int64(math.Round(row.MemoryAvg)),
				MemoryMax:         int64(math.Round(row.MemoryMax)),
				Load1Avg:          row.Load1Avg,
				Load1Max:          row.Load1Max,
				NetRxBytesSum:     row.NetRxBytesSum,
				NetTxBytesSum:     row.NetTxBytesSum,
				TCPEstablishedAvg: row.TCPEstablishedAvg,
				SampleCount:       int32(row.SampleCount),
			})
		}
		if err := tx.Create(&records).Error; err != nil {
			return err
		}
		return nil
	})
}

// alignTo 将时间戳向下对齐到指定窗口
func alignTo(timestamp, window int64) int64 {
	if window <= 0 {
		return timestamp
	}
	return timestamp - timestamp%window
}

// beginningOfMonthUTC 返回 UTC 时区当月第一天 0 点
func beginningOfMonthUTC(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}
