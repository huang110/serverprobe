package model

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestDB 创建内存 SQLite 测试数据库。
// 使用单连接保证 :memory: 数据库在测试期间不丢失。
func setupTestDB(t *testing.T) *gorm.DB {
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

	return db
}

func TestAutoMigrate(t *testing.T) {
	db := setupTestDB(t)

	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() failed: %v", err)
	}

	tables := []string{
		"agents",
		"agent_labels",
		"metrics_aggregated",
		"metrics_disk",
		"metrics_network",
		"alert_rules",
		"alert_records",
	}
	for _, table := range tables {
		if !db.Migrator().HasTable(table) {
			t.Errorf("table %s does not exist", table)
		}
	}

	currentPartition := GetMetricsRawTableName(time.Now())
	if !db.Migrator().HasTable(currentPartition) {
		t.Errorf("partition table %s does not exist", currentPartition)
	}

	nextPartition := GetMetricsRawTableName(time.Now().AddDate(0, 1, 0))
	if !db.Migrator().HasTable(nextPartition) {
		t.Errorf("next month partition table %s does not exist", nextPartition)
	}

	// 分区表联合索引检查（SQLite 索引名带月份后缀）
	partitionSuffix := strings.TrimPrefix(currentPartition, "metrics_raw_")
	agentTimeIndex := "idx_agent_time_" + partitionSuffix
	if !db.Migrator().HasIndex(currentPartition, agentTimeIndex) {
		t.Errorf("index %s on %s does not exist", agentTimeIndex, currentPartition)
	}
	columns := sqliteIndexColumns(t, db, agentTimeIndex)
	if len(columns) != 2 || columns[0] != "agent_id" || columns[1] != "timestamp" {
		t.Errorf("index %s columns = %v, want [agent_id timestamp]", agentTimeIndex, columns)
	}

	// 基础表索引检查
	indexes := []struct {
		table string
		index string
	}{
		{"agents", "idx_hostname"},
		{"agents", "idx_ip"},
		{"agents", "idx_token_expires_at"},
		{"agents", "idx_last_heartbeat_at"},
		{"agents", "idx_status"},
		{"agent_labels", "idx_agent_key"},
		{"metrics_aggregated", "idx_agent_granularity_time"},
		{"metrics_disk", "idx_agent_mount_time"},
		{"metrics_network", "idx_agent_iface_time"},
		{"alert_rules", "idx_agent_id"},
		{"alert_rules", "idx_enabled"},
		{"alert_records", "idx_rule_id"},
		{"alert_records", "idx_alert_records_agent_id"},
		{"alert_records", "idx_severity"},
		{"alert_records", "idx_alert_records_status"},
		{"alert_records", "idx_fired_at"},
	}
	for _, item := range indexes {
		if !db.Migrator().HasIndex(item.table, item.index) {
			t.Errorf("index %s on %s does not exist", item.index, item.table)
		}
	}
}

func TestAutoMigrateIdempotent(t *testing.T) {
	db := setupTestDB(t)

	if err := AutoMigrate(db); err != nil {
		t.Fatalf("first AutoMigrate() failed: %v", err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("second AutoMigrate() failed: %v", err)
	}
}

func TestGetMetricsRawTableName(t *testing.T) {
	tests := []struct {
		time time.Time
		want string
	}{
		{time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC), "metrics_raw_202601"},
		{time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), "metrics_raw_202612"},
		{time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC), "metrics_raw_202702"},
	}

	for _, test := range tests {
		if got := GetMetricsRawTableName(test.time); got != test.want {
			t.Errorf("GetMetricsRawTableName(%v) = %q, want %q", test.time, got, test.want)
		}
	}
}

func TestCreateMetricsRawPartitionsValidation(t *testing.T) {
	db := setupTestDB(t)

	if err := CreateMetricsRawPartitions(nil, time.Now(), 1); err == nil {
		t.Error("nil database should return an error")
	}
	if err := CreateMetricsRawPartitions(db, time.Now(), 0); err == nil {
		t.Error("zero count should return an error")
	}
}

// sqliteIndexColumns 返回 SQLite 索引包含的列名
func sqliteIndexColumns(t *testing.T, db *gorm.DB, index string) []string {
	t.Helper()

	var rows []struct {
		Name string
	}
	query := fmt.Sprintf("PRAGMA index_info('%s')", strings.ReplaceAll(index, "'", "''"))
	if err := db.Raw(query).Scan(&rows).Error; err != nil {
		t.Fatalf("failed to inspect index %s: %v", index, err)
	}

	columns := make([]string, 0, len(rows))
	for _, row := range rows {
		columns = append(columns, row.Name)
	}
	return columns
}
