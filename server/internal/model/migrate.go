package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// AutoMigrate 自动迁移所有基础表，并创建当前月和下个月的原始指标分区表。
func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is required")
	}

	if err := db.AutoMigrate(
		&Agent{},
		&AgentLabel{},
		&MetricsAggregated{},
		&MetricsDisk{},
		&MetricsNetwork{},
		&MetricsLatency{},
		&AlertRule{},
		&AlertRecord{},
	); err != nil {
		return fmt.Errorf("failed to migrate base tables: %w", err)
	}

	if err := CreateMetricsRawPartitions(db, time.Now(), 2); err != nil {
		return fmt.Errorf("failed to create metrics_raw partitions: %w", err)
	}

	return nil
}

// CreateMetricsRawPartitions 从 start 所在月份开始，连续创建 count 个原始指标分区表。
// 分区表索引单独创建：MySQL 的索引名是表级作用域，而 SQLite 等数据库要求
// 索引名在库内全局唯一，因此非 MySQL 数据库会附加月份后缀。
func CreateMetricsRawPartitions(db *gorm.DB, start time.Time, count int) error {
	if db == nil {
		return errors.New("database is required")
	}
	if count <= 0 {
		return errors.New("partition count must be positive")
	}

	for i := 0; i < count; i++ {
		month := start.AddDate(0, i, 0)
		tableName := GetMetricsRawTableName(month)

		// 使用动态表名迁移；直接 AutoMigrate(&MetricsRaw{}) 只会创建默认表 metrics_raw。
		if err := db.Table(tableName).AutoMigrate(&MetricsRaw{}); err != nil {
			return fmt.Errorf("failed to create partition %s: %w", tableName, err)
		}
		if err := createMetricsRawIndexes(db, tableName); err != nil {
			return err
		}
	}
	return nil
}

// createMetricsRawIndexes 为分区表创建 (agent_id, timestamp) 联合索引和 created_at 索引。
func createMetricsRawIndexes(db *gorm.DB, tableName string) error {
	indexSuffix := ""
	if db.Dialector.Name() != "mysql" {
		indexSuffix = "_" + strings.TrimPrefix(tableName, "metrics_raw_")
	}

	indexes := []struct {
		name    string
		columns string
	}{
		{name: "idx_agent_time" + indexSuffix, columns: "agent_id, timestamp"},
		{name: "idx_created_at" + indexSuffix, columns: "created_at"},
	}

	for _, index := range indexes {
		if db.Migrator().HasIndex(tableName, index.name) {
			continue
		}

		statement := fmt.Sprintf(
			"CREATE INDEX %s ON %s (%s)",
			quoteIdentifier(db, index.name),
			quoteIdentifier(db, tableName),
			index.columns,
		)
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("failed to create index %s on %s: %w", index.name, tableName, err)
		}
	}
	return nil
}

// quoteIdentifier 根据数据库方言引用标识符。
func quoteIdentifier(db *gorm.DB, identifier string) string {
	switch db.Dialector.Name() {
	case "mysql", "sqlite":
		return "`" + identifier + "`"
	default:
		return `"` + identifier + `"`
	}
}

// GetMetricsRawTableName 返回指定时间对应的原始指标分区表名。
func GetMetricsRawTableName(t time.Time) string {
	return fmt.Sprintf("metrics_raw_%s", t.Format("200601"))
}
