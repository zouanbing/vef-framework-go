package monitor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/monitor"
)

// withCgroupFS points cgroupBasePath at a fresh temp dir and writes the given
// relative files into it, restoring the original base on cleanup.
func withCgroupFS(t *testing.T, files map[string]string) {
	t.Helper()

	dir := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}

	originalPath := cgroupBasePath
	originalReadable := cgroupReadable
	cgroupBasePath = dir
	cgroupReadable = true // exercise the parsers regardless of the test host OS
	t.Cleanup(func() {
		cgroupBasePath = originalPath
		cgroupReadable = originalReadable
	})
}

// TestCgroupReadableGate confirms non-Linux hosts short-circuit to host fallback
// even when cgroup files happen to exist.
func TestCgroupReadableGate(t *testing.T) {
	withCgroupFS(t, map[string]string{
		relV2MemMax:     "536870912\n",
		relV2MemCurrent: "1\n",
		relV2CPUMax:     "200000 100000\n",
	})

	cgroupReadable = false // simulate macOS / *BSD / Windows

	_, _, memOK := cgroupMemoryLimit()
	_, cpuOK := cgroupCPUCores()
	assert.False(t, memOK, "非 Linux 应短路回退,不读 cgroup")
	assert.False(t, cpuOK, "非 Linux 应短路回退,不读 cgroup")
}

func TestCgroupMemoryLimit(t *testing.T) {
	t.Run("空目录(裸机/非Linux)回退", func(t *testing.T) {
		withCgroupFS(t, map[string]string{})
		_, _, ok := cgroupMemoryLimit()
		assert.False(t, ok, "无 cgroup 文件时应回退宿主")
	})

	t.Run("v2 max 视为无限额", func(t *testing.T) {
		withCgroupFS(t, map[string]string{relV2MemMax: "max\n"})
		_, _, ok := cgroupMemoryLimit()
		assert.False(t, ok, "memory.max=max 应回退宿主")
	})

	t.Run("v2 有限额:用量扣除可回收缓存", func(t *testing.T) {
		withCgroupFS(t, map[string]string{
			relV2MemMax:     "536870912\n",           // 512Mi
			relV2MemCurrent: "200000000\n",           // ~200MB current
			relV2MemStat:    "inactive_file 50000000\nanon 10\n", // 50MB reclaimable
		})
		limit, used, ok := cgroupMemoryLimit()
		require.True(t, ok)
		assert.Equal(t, uint64(536870912), limit)
		assert.Equal(t, uint64(150000000), used, "工作集=current-inactive_file")
	})

	t.Run("v2 哨兵大值回退", func(t *testing.T) {
		withCgroupFS(t, map[string]string{relV2MemMax: "9223372036854771712\n"})
		_, _, ok := cgroupMemoryLimit()
		assert.False(t, ok, "超大哨兵值应视为无限额")
	})

	t.Run("v1 有限额", func(t *testing.T) {
		withCgroupFS(t, map[string]string{
			relV1MemLimit: "268435456\n", // 256Mi
			relV1MemUsage: "100000000\n",
			relV1MemStat:  "total_inactive_file 20000000\n",
		})
		limit, used, ok := cgroupMemoryLimit()
		require.True(t, ok)
		assert.Equal(t, uint64(268435456), limit)
		assert.Equal(t, uint64(80000000), used)
	})

	t.Run("畸形内容回退", func(t *testing.T) {
		withCgroupFS(t, map[string]string{relV2MemMax: "not-a-number\n"})
		_, _, ok := cgroupMemoryLimit()
		assert.False(t, ok, "非数字内容应回退,不 panic")
	})

	t.Run("有限额但 current 缺失回退", func(t *testing.T) {
		withCgroupFS(t, map[string]string{relV2MemMax: "536870912\n"})
		_, _, ok := cgroupMemoryLimit()
		assert.False(t, ok, "读不到 current 时回退")
	})
}

func TestCgroupCPUCores(t *testing.T) {
	t.Run("空目录回退", func(t *testing.T) {
		withCgroupFS(t, map[string]string{})
		_, ok := cgroupCPUCores()
		assert.False(t, ok)
	})

	t.Run("v2 max 无配额回退", func(t *testing.T) {
		withCgroupFS(t, map[string]string{relV2CPUMax: "max 100000\n"})
		_, ok := cgroupCPUCores()
		assert.False(t, ok, "cpu.max=max 应回退宿主核数")
	})

	t.Run("v2 配额=2核", func(t *testing.T) {
		withCgroupFS(t, map[string]string{relV2CPUMax: "200000 100000\n"})
		cores, ok := cgroupCPUCores()
		require.True(t, ok)
		assert.InDelta(t, 2.0, cores, 1e-9)
	})

	t.Run("v2 分数核 1.5", func(t *testing.T) {
		withCgroupFS(t, map[string]string{relV2CPUMax: "150000 100000\n"})
		cores, ok := cgroupCPUCores()
		require.True(t, ok)
		assert.InDelta(t, 1.5, cores, 1e-9)
	})

	t.Run("v1 quota=-1 无限额回退", func(t *testing.T) {
		withCgroupFS(t, map[string]string{
			relV1CPUQuota:  "-1\n",
			relV1CPUPeriod: "100000\n",
		})
		_, ok := cgroupCPUCores()
		assert.False(t, ok)
	})

	t.Run("v1 配额=4核", func(t *testing.T) {
		withCgroupFS(t, map[string]string{
			relV1CPUQuota:  "400000\n",
			relV1CPUPeriod: "100000\n",
		})
		cores, ok := cgroupCPUCores()
		require.True(t, ok)
		assert.InDelta(t, 4.0, cores, 1e-9)
	})

	t.Run("period=0 不除零", func(t *testing.T) {
		withCgroupFS(t, map[string]string{relV2CPUMax: "100000 0\n"})
		_, ok := cgroupCPUCores()
		assert.False(t, ok, "period 为 0 时安全回退,不 panic")
	})

	t.Run("畸形字段数回退", func(t *testing.T) {
		withCgroupFS(t, map[string]string{relV2CPUMax: "garbage\n"})
		_, ok := cgroupCPUCores()
		assert.False(t, ok)
	})
}

func TestCgroupCPUUsageMicros(t *testing.T) {
	t.Run("v2 usage_usec", func(t *testing.T) {
		withCgroupFS(t, map[string]string{relV2CPUStat: "usage_usec 123456\nuser_usec 1\n"})
		v, ok := cgroupCPUUsageMicros()
		require.True(t, ok)
		assert.Equal(t, uint64(123456), v)
	})

	t.Run("v1 纳秒转微秒", func(t *testing.T) {
		withCgroupFS(t, map[string]string{relV1CPUUsage: "1000000\n"}) // 1e6 ns = 1000 us
		v, ok := cgroupCPUUsageMicros()
		require.True(t, ok)
		assert.Equal(t, uint64(1000), v)
	})

	t.Run("空目录回退", func(t *testing.T) {
		withCgroupFS(t, map[string]string{})
		_, ok := cgroupCPUUsageMicros()
		assert.False(t, ok)
	})
}

func TestSubClamp(t *testing.T) {
	assert.Equal(t, uint64(0), subClamp(5, 10), "下溢应钳为 0")
	assert.Equal(t, uint64(5), subClamp(10, 5))
}

func TestApplyCgroupMemory(t *testing.T) {
	const hostTotal = uint64(8) << 30 // 8 GiB host

	t.Run("限额小于宿主总量则覆盖", func(t *testing.T) {
		withCgroupFS(t, map[string]string{
			relV2MemMax:     "536870912\n", // 512Mi limit
			relV2MemCurrent: "268435456\n", // 256Mi current
			relV2MemStat:    "inactive_file 0\n",
		})

		v := &monitor.VirtualMemory{Total: hostTotal, Used: 1 << 30, UsedPercent: 12.5}
		applyCgroupMemory(v)

		assert.Equal(t, uint64(536870912), v.Total, "应改为容器限额")
		assert.Equal(t, uint64(268435456), v.Used)
		assert.InDelta(t, 50.0, v.UsedPercent, 0.01, "占比应按容器限额算")
	})

	t.Run("限额不小于宿主总量则保持宿主", func(t *testing.T) {
		withCgroupFS(t, map[string]string{
			relV2MemMax:     "17179869184\n", // 16Gi > 8Gi host
			relV2MemCurrent: "1\n",
			relV2MemStat:    "inactive_file 0\n",
		})

		v := &monitor.VirtualMemory{Total: hostTotal, Used: 1 << 30, UsedPercent: 12.5}
		applyCgroupMemory(v)

		assert.Equal(t, hostTotal, v.Total, "限额≥宿主总量应保持宿主值")
		assert.InDelta(t, 12.5, v.UsedPercent, 0.01)
	})

	t.Run("无 cgroup 保持宿主", func(t *testing.T) {
		withCgroupFS(t, map[string]string{})

		v := &monitor.VirtualMemory{Total: hostTotal, Used: 1 << 30, UsedPercent: 12.5}
		applyCgroupMemory(v)

		assert.Equal(t, hostTotal, v.Total)
	})

	t.Run("nil 不 panic", func(t *testing.T) {
		assert.NotPanics(t, func() { applyCgroupMemory(nil) })
	})
}
