package monitor

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// cgroup.go detects container CPU/memory limits from the cgroup filesystem
// (v2 unified hierarchy first, then v1). Every reader returns ok=false on any
// problem — not Linux, no limit configured, missing files, or malformed content —
// so callers transparently fall back to host-level metrics. Nothing here ever
// panics or returns an error; robustness of the bare-metal path is the priority.

// cgroupBasePath is the cgroup mount root. It is a variable rather than a
// constant only so tests can point it at a temporary directory.
var cgroupBasePath = "/sys/fs/cgroup"

// cgroupReadable is true only on Linux, the sole platform with a cgroup
// filesystem. cgroups are a Linux-specific kernel feature — macOS, the *BSDs,
// Solaris and Windows have no /sys/fs/cgroup — so on every other platform the
// readers short-circuit and callers fall back to host metrics. It is a variable
// so tests can exercise the parsers on a non-Linux CI machine.
var cgroupReadable = runtime.GOOS == "linux"

// cgroup file locations, relative to cgroupBasePath (v2 unified, then v1).
const (
	relV2MemMax     = "memory.max"
	relV2MemCurrent = "memory.current"
	relV2MemStat    = "memory.stat"
	relV2CPUMax     = "cpu.max"
	relV2CPUStat    = "cpu.stat"

	relV1MemLimit  = "memory/memory.limit_in_bytes"
	relV1MemUsage  = "memory/memory.usage_in_bytes"
	relV1MemStat   = "memory/memory.stat"
	relV1CPUQuota  = "cpu/cpu.cfs_quota_us"
	relV1CPUPeriod = "cpu/cpu.cfs_period_us"
	relV1CPUUsage  = "cpuacct/cpuacct.usage" // nanoseconds
)

// cgroupUnlimited is the threshold above which a cgroup v1 byte limit is treated
// as "no limit": the kernel reports unlimited as a huge page-aligned sentinel
// (e.g. 0x7FFFFFFFFFFFF000), far above any real machine's RAM.
const cgroupUnlimited = uint64(1) << 62

func cgPath(rel string) string {
	return filepath.Join(cgroupBasePath, rel)
}

// readTrimmed returns the trimmed file content and whether it was read non-empty.
func readTrimmed(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	s := strings.TrimSpace(string(b))
	if s == "" {
		return "", false
	}

	return s, true
}

// readUint reads a single unsigned integer from a cgroup file.
func readUint(path string) (uint64, bool) {
	s, ok := readTrimmed(path)
	if !ok {
		return 0, false
	}

	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, false
	}

	return v, true
}

// memStatField extracts a "<key> <value>" field from a cgroup stat file.
func memStatField(path, key string) (uint64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}

	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == key {
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0, false
			}

			return v, true
		}
	}

	return 0, false
}

// cgroupMemoryLimit returns the container memory limit and working-set usage in
// bytes. ok is false when no finite limit applies (fall back to host). Usage
// excludes reclaimable page cache (inactive_file) to match the kernel's OOM
// accounting, mirroring how `docker stats` reports memory.
func cgroupMemoryLimit() (limit, used uint64, ok bool) {
	if !cgroupReadable {
		return 0, 0, false
	}

	// cgroup v2.
	if raw, found := readTrimmed(cgPath(relV2MemMax)); found {
		if raw == "max" {
			return 0, 0, false
		}

		lim, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || lim == 0 || lim >= cgroupUnlimited {
			return 0, 0, false
		}

		cur, curOK := readUint(cgPath(relV2MemCurrent))
		if !curOK {
			return 0, 0, false
		}

		inactive, _ := memStatField(cgPath(relV2MemStat), "inactive_file")

		return lim, subClamp(cur, inactive), true
	}

	// cgroup v1.
	if lim, found := readUint(cgPath(relV1MemLimit)); found {
		if lim == 0 || lim >= cgroupUnlimited {
			return 0, 0, false
		}

		cur, curOK := readUint(cgPath(relV1MemUsage))
		if !curOK {
			return 0, 0, false
		}

		inactive, _ := memStatField(cgPath(relV1MemStat), "total_inactive_file")

		return lim, subClamp(cur, inactive), true
	}

	return 0, 0, false
}

// cgroupCPUCores returns the effective (possibly fractional) CPU core count from
// the CFS quota (quota/period). ok is false when no quota is set (unlimited) or
// on any parse error, so the caller keeps the host core count.
func cgroupCPUCores() (float64, bool) {
	if !cgroupReadable {
		return 0, false
	}

	// cgroup v2: "cpu.max" is "<quota> <period>" or "max <period>".
	if raw, found := readTrimmed(cgPath(relV2CPUMax)); found {
		fields := strings.Fields(raw)
		if len(fields) != 2 || fields[0] == "max" {
			return 0, false
		}

		quota, err1 := strconv.ParseFloat(fields[0], 64)
		period, err2 := strconv.ParseFloat(fields[1], 64)
		if err1 != nil || err2 != nil || quota <= 0 || period <= 0 {
			return 0, false
		}

		return quota / period, true
	}

	// cgroup v1: a quota of -1 means unlimited.
	quotaRaw, ok1 := readTrimmed(cgPath(relV1CPUQuota))
	periodRaw, ok2 := readTrimmed(cgPath(relV1CPUPeriod))
	if !ok1 || !ok2 {
		return 0, false
	}

	quota, err1 := strconv.ParseInt(quotaRaw, 10, 64)
	period, err2 := strconv.ParseInt(periodRaw, 10, 64)
	if err1 != nil || err2 != nil || quota <= 0 || period <= 0 {
		return 0, false
	}

	return float64(quota) / float64(period), true
}

// cgroupCPUUsageMicros returns the cgroup's cumulative CPU time in microseconds.
// Two readings across an interval give the container's own CPU utilisation.
func cgroupCPUUsageMicros() (uint64, bool) {
	if !cgroupReadable {
		return 0, false
	}

	// cgroup v2: "cpu.stat" contains "usage_usec <n>".
	if v, ok := memStatField(cgPath(relV2CPUStat), "usage_usec"); ok {
		return v, true
	}

	// cgroup v1: cpuacct.usage is nanoseconds.
	if ns, ok := readUint(cgPath(relV1CPUUsage)); ok {
		return ns / 1000, true
	}

	return 0, false
}

// subClamp returns a-b, clamped at zero to avoid unsigned underflow.
func subClamp(a, b uint64) uint64 {
	if b > a {
		return 0
	}

	return a - b
}
