package collector

import (
	"math"
	"testing"

	"github.com/shirou/gopsutil/v3/disk"
)

// TestDiskCollectorFiltering 验证文件系统和挂载点过滤规则
func TestDiskCollectorFiltering(t *testing.T) {
	c := NewDiskCollector(
		WithDiskMountPoints([]string{" / ", "", "/data*"}),
		WithExcludedFSTypes([]string{" XFS ", ""}),
	)

	mountTests := []struct {
		mountPoint string
		want       bool
	}{
		{"/", true},
		{"/data", true},
		{"/data1", true},
		{"/home", false},
		{"/var/lib", false},
	}

	for _, test := range mountTests {
		if got := c.shouldMonitorMount(test.mountPoint); got != test.want {
			t.Errorf("shouldMonitorMount(%q) = %v, want %v", test.mountPoint, got, test.want)
		}
	}

	fsTests := []struct {
		fsType string
		want   bool
	}{
		{"tmpfs", true},
		{"DEVTMPFS", true},
		{"overlay", true},
		{"xfs", true},
		{"ext4", false},
		{"ntfs", false},
	}

	for _, test := range fsTests {
		if got := c.isExcludedFS(test.fsType); got != test.want {
			t.Errorf("isExcludedFS(%q) = %v, want %v", test.fsType, got, test.want)
		}
	}
}

// TestCalculateDiskIO 验证两次采样之间的 IO 速率、IOPS 和 IO 使用率计算
func TestCalculateDiskIO(t *testing.T) {
	previous := disk.IOCountersStat{
		ReadBytes:  1000,
		WriteBytes: 2000,
		ReadCount:  10,
		WriteCount: 20,
		IoTime:     100,
	}
	current := disk.IOCountersStat{
		ReadBytes:  3000,
		WriteBytes: 6000,
		ReadCount:  30,
		WriteCount: 60,
		IoTime:     600,
	}

	got := calculateDiskIO(previous, current, 2)

	assertFloatEqual(t, "ReadBytesPerSec", got.ReadBytesPerSec, 1000)
	assertFloatEqual(t, "WriteBytesPerSec", got.WriteBytesPerSec, 2000)
	assertFloatEqual(t, "ReadIOPS", got.ReadIOPS, 10)
	assertFloatEqual(t, "WriteIOPS", got.WriteIOPS, 20)
	assertFloatEqual(t, "IOUtilPercent", got.IOUtilPercent, 25)
}

// TestCalculateDiskIOReset 验证计数器重置和无效采样间隔的处理
func TestCalculateDiskIOReset(t *testing.T) {
	previous := disk.IOCountersStat{ReadBytes: 5000, WriteBytes: 5000, IoTime: 1000}
	current := disk.IOCountersStat{ReadBytes: 100, WriteBytes: 100, IoTime: 10}

	got := calculateDiskIO(previous, current, 1)
	if got.ReadBytesPerSec != 0 || got.WriteBytesPerSec != 0 || got.IOUtilPercent != 0 {
		t.Errorf("counter reset should produce zero rates, got %+v", got)
	}

	zeroDuration := calculateDiskIO(previous, current, 0)
	if zeroDuration != (diskIORates{}) {
		t.Errorf("zero duration should produce zero rates, got %+v", zeroDuration)
	}
}

// TestFindIOStat 验证设备名到 IO 计数器名称的匹配
func TestFindIOStat(t *testing.T) {
	stats := map[string]disk.IOCountersStat{
		"sda1": {Name: "sda1", ReadBytes: 123},
	}

	stat, ok := findIOStat("/dev/sda1", stats)
	if !ok {
		t.Fatal("expected to find IO stat for /dev/sda1")
	}
	if stat.ReadBytes != 123 {
		t.Errorf("unexpected IO stat: %+v", stat)
	}

	if _, ok := findIOStat("/dev/sdb1", stats); ok {
		t.Error("did not expect to find IO stat for /dev/sdb1")
	}
}

// TestDiskCollectorCollect 对真实系统执行一次采集，验证返回值的基本约束
func TestDiskCollectorCollect(t *testing.T) {
	c := NewDiskCollector()

	metrics, err := c.Collect()
	if err != nil {
		t.Fatalf("Collect() returned error: %v", err)
	}
	if len(metrics) == 0 {
		t.Skip("no physical disk partitions available in this environment")
	}

	for _, metric := range metrics {
		if metric.MountPoint == "" {
			t.Error("mount point should not be empty")
		}
		if metric.Total < 0 || metric.Used < 0 || metric.Free < 0 {
			t.Errorf("disk sizes should not be negative: %+v", metric)
		}
		if metric.UsagePercent < 0 || metric.UsagePercent > 100 {
			t.Errorf("usage percent out of range: %f", metric.UsagePercent)
		}
		if metric.InodeUsagePercent < 0 || metric.InodeUsagePercent > 100 {
			t.Errorf("inode usage percent out of range: %f", metric.InodeUsagePercent)
		}
		if metric.ReadBytesPerSec < 0 || metric.WriteBytesPerSec < 0 {
			t.Errorf("IO rates should not be negative: %+v", metric)
		}
		if metric.ReadIOPS < 0 || metric.WriteIOPS < 0 {
			t.Errorf("IOPS should not be negative: %+v", metric)
		}
		if metric.IOUtilPercent < 0 || metric.IOUtilPercent > 100 {
			t.Errorf("IO utilization out of range: %f", metric.IOUtilPercent)
		}
	}

	// 第二次采集会基于上次快照计算速率，速率不应为负数
	if _, err := c.Collect(); err != nil {
		t.Fatalf("second Collect() returned error: %v", err)
	}
}

// assertFloatEqual 比较两个浮点数
func assertFloatEqual(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %f, want %f", name, got, want)
	}
}
