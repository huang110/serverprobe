package aggregator

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/huang110/serverprobe/server/internal/model"
)

func setupAggregatorTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if closeErr := sqlDB.Close(); closeErr != nil {
			t.Errorf("failed to close database: %v", closeErr)
		}
	})

	if err := model.AutoMigrate(db); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}
	return db
}

func TestAlignTo(t *testing.T) {
	tests := []struct {
		timestamp int64
		window    int64
		want      int64
	}{
		{301, 300, 300},
		{600, 300, 600},
		{599, 300, 300},
		{0, 300, 0},
		{12345, 3600, 10800},
	}

	for _, test := range tests {
		if got := alignTo(test.timestamp, test.window); got != test.want {
			t.Errorf("alignTo(%d, %d) = %d, want %d", test.timestamp, test.window, got, test.want)
		}
	}
}

func TestAggregate5m(t *testing.T) {
	db := setupAggregatorTestDB(t)
	aggregator := NewAggregator(db)

	now := time.Date(time.Now().UTC().Year(), time.Now().UTC().Month(), 15, 12, 0, 0, 0, time.UTC)
	windowEnd := alignTo(now.Unix(), fiveMinuteSeconds)
	windowStart := windowEnd - fiveMinuteSeconds
	tableName := model.GetMetricsRawTableName(now)

	rawRows := []model.MetricsRaw{
		{
			AgentID:       "agent-1",
			Timestamp:     windowStart + 10,
			CPUUsage:      10,
			MemoryPercent: 50,
			Load1:         1,
			NetRxBytes:    100,
			NetTxBytes:    200,
			TCPEstab:      5,
		},
		{
			AgentID:       "agent-1",
			Timestamp:     windowStart + 70,
			CPUUsage:      30,
			MemoryPercent: 70,
			Load1:         3,
			NetRxBytes:    300,
			NetTxBytes:    400,
			TCPEstab:      15,
		},
		{
			AgentID:   "agent-1",
			Timestamp: windowStart + fiveMinuteSeconds, // 下一个窗口，不应被聚合
			CPUUsage:  100,
		},
	}
	for i := range rawRows {
		if err := db.Table(tableName).Create(&rawRows[i]).Error; err != nil {
			t.Fatalf("failed to insert raw metrics: %v", err)
		}
	}

	count, err := aggregator.Aggregate5m(now)
	if err != nil {
		t.Fatalf("Aggregate5m() failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("aggregated row count = %d, want 1", count)
	}

	var row model.MetricsAggregated
	if err := db.Where("agent_id = ? AND granularity = ?", "agent-1", "5m").First(&row).Error; err != nil {
		t.Fatalf("failed to query aggregated row: %v", err)
	}
	if row.Timestamp != windowStart {
		t.Errorf("timestamp = %d, want %d", row.Timestamp, windowStart)
	}
	if row.CPUAvg != 20 || row.CPUMax != 30 {
		t.Errorf("cpu avg/max = %.2f/%.2f, want 20/30", row.CPUAvg, row.CPUMax)
	}
	if row.MemoryAvg != 60 || row.Load1Avg != 2 {
		t.Errorf("memory/load avg = %d/%.2f, want 60/2", row.MemoryAvg, row.Load1Avg)
	}
	if row.NetRxBytesSum != 400 || row.NetTxBytesSum != 600 {
		t.Errorf("net sums = %d/%d, want 400/600", row.NetRxBytesSum, row.NetTxBytesSum)
	}
	if row.TCPEstablishedAvg != 10 {
		t.Errorf("tcp avg = %.2f, want 10", row.TCPEstablishedAvg)
	}
	if row.SampleCount != 2 {
		t.Errorf("sample count = %d, want 2", row.SampleCount)
	}

	// 重复聚合不应产生重复数据
	if _, err := aggregator.Aggregate5m(now); err != nil {
		t.Fatalf("second Aggregate5m() failed: %v", err)
	}
	var countRows int64
	db.Model(&model.MetricsAggregated{}).Where("granularity = ?", "5m").Count(&countRows)
	if countRows != 1 {
		t.Errorf("aggregated row count after second run = %d, want 1", countRows)
	}
}

func TestAggregate1hAnd1d(t *testing.T) {
	db := setupAggregatorTestDB(t)
	aggregator := NewAggregator(db)

	now := time.Date(time.Now().UTC().Year(), time.Now().UTC().Month(), 15, 12, 0, 0, 0, time.UTC)
	hourStart := alignTo(now.Unix(), oneHourSeconds) - oneHourSeconds

	for i := 0; i < 2; i++ {
		row := model.MetricsAggregated{
			AgentID:     "agent-1",
			Granularity: "5m",
			Timestamp:   hourStart + int64(i)*300,
			CPUAvg:      float64(10 + i*10),
			CPUMax:      float64(20 + i*10),
			MemoryAvg:   50,
			MemoryMax:   60,
			SampleCount: 2,
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("failed to insert 5m rows: %v", err)
		}
	}

	if _, err := aggregator.Aggregate1h(now); err != nil {
		t.Fatalf("Aggregate1h() failed: %v", err)
	}

	var hourly model.MetricsAggregated
	if err := db.Where("granularity = ?", "1h").First(&hourly).Error; err != nil {
		t.Fatalf("failed to query 1h row: %v", err)
	}
	if hourly.CPUAvg != 15 || hourly.CPUMax != 30 || hourly.SampleCount != 4 {
		t.Errorf("unexpected 1h aggregation: %+v", hourly)
	}

	// 1d 聚合
	dayStart := alignTo(now.Unix(), oneDaySeconds) - oneDaySeconds
	dailySource := model.MetricsAggregated{
		AgentID:     "agent-1",
		Granularity: "1h",
		Timestamp:   dayStart,
		CPUAvg:      25,
		CPUMax:      40,
		MemoryAvg:   55,
		MemoryMax:   65,
		SampleCount: 10,
	}
	if err := db.Create(&dailySource).Error; err != nil {
		t.Fatalf("failed to insert 1h row: %v", err)
	}

	if _, err := aggregator.Aggregate1d(now); err != nil {
		t.Fatalf("Aggregate1d() failed: %v", err)
	}
	var daily model.MetricsAggregated
	if err := db.Where("granularity = ?", "1d").First(&daily).Error; err != nil {
		t.Fatalf("failed to query 1d row: %v", err)
	}
	if daily.CPUAvg != 25 || daily.SampleCount != 10 {
		t.Errorf("unexpected 1d aggregation: %+v", daily)
	}
}

func TestCleanOldData(t *testing.T) {
	db := setupAggregatorTestDB(t)
	cleaner := NewCleaner(db)
	now := time.Now().UTC()

	oldTime := time.Date(now.Year(), now.Month(), 15, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
	if err := model.CreateMetricsRawPartitions(db, oldTime, 1); err != nil {
		t.Fatalf("failed to create old partition: %v", err)
	}
	oldTable := model.GetMetricsRawTableName(oldTime)
	if err := db.Table(oldTable).Create(&model.MetricsRaw{
		AgentID:   "agent-1",
		Timestamp: oldTime.Unix(),
		CPUUsage:  10,
	}).Error; err != nil {
		t.Fatalf("failed to insert old raw metrics: %v", err)
	}
	currentTable := model.GetMetricsRawTableName(now)
	if err := db.Table(currentTable).Create(&model.MetricsRaw{
		AgentID:   "agent-1",
		Timestamp: now.Unix(),
		CPUUsage:  20,
	}).Error; err != nil {
		t.Fatalf("failed to insert current raw metrics: %v", err)
	}

	oldAggregated := []model.MetricsAggregated{
		{AgentID: "agent-1", Granularity: "5m", Timestamp: now.AddDate(0, 0, -40).Unix(), CPUAvg: 1},
		{AgentID: "agent-1", Granularity: "5m", Timestamp: now.AddDate(0, 0, -5).Unix(), CPUAvg: 2},
		{AgentID: "agent-1", Granularity: "1h", Timestamp: now.AddDate(0, 0, -100).Unix(), CPUAvg: 3},
		{AgentID: "agent-1", Granularity: "1d", Timestamp: now.AddDate(0, 0, -400).Unix(), CPUAvg: 4},
	}
	for i := range oldAggregated {
		if err := db.Create(&oldAggregated[i]).Error; err != nil {
			t.Fatalf("failed to insert aggregated rows: %v", err)
		}
	}

	oldDisk := model.MetricsDisk{AgentID: "agent-1", MountPoint: "/", Timestamp: oldTime.Unix()}
	currentDisk := model.MetricsDisk{AgentID: "agent-1", MountPoint: "/", Timestamp: now.Unix()}
	if err := db.Create(&oldDisk).Error; err != nil {
		t.Fatalf("failed to insert old disk metrics: %v", err)
	}
	if err := db.Create(&currentDisk).Error; err != nil {
		t.Fatalf("failed to insert current disk metrics: %v", err)
	}

	if err := cleaner.CleanOldData(now); err != nil {
		t.Fatalf("CleanOldData() failed: %v", err)
	}

	// 旧原始数据被删除，新数据保留
	var rawCount int64
	db.Table(oldTable).Where("agent_id = ?", "agent-1").Count(&rawCount)
	if rawCount != 0 {
		t.Errorf("old raw metrics count = %d, want 0", rawCount)
	}
	db.Table(currentTable).Where("agent_id = ?", "agent-1").Count(&rawCount)
	if rawCount != 1 {
		t.Errorf("current raw metrics count = %d, want 1", rawCount)
	}

	// 5m/1h/1d 过期数据被删除
	var aggregatedCount int64
	db.Model(&model.MetricsAggregated{}).Where("granularity = ?", "5m").Count(&aggregatedCount)
	if aggregatedCount != 1 {
		t.Errorf("5m row count = %d, want 1", aggregatedCount)
	}
	db.Model(&model.MetricsAggregated{}).Where("granularity = ?", "1h").Count(&aggregatedCount)
	if aggregatedCount != 0 {
		t.Errorf("1h row count = %d, want 0", aggregatedCount)
	}
	db.Model(&model.MetricsAggregated{}).Where("granularity = ?", "1d").Count(&aggregatedCount)
	if aggregatedCount != 0 {
		t.Errorf("1d row count = %d, want 0", aggregatedCount)
	}

	var diskCount int64
	db.Model(&model.MetricsDisk{}).Count(&diskCount)
	if diskCount != 1 {
		t.Errorf("disk metric count = %d, want 1", diskCount)
	}
}
