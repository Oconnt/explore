package prowler

import (
	"fmt"
	"io/ioutil"
	"sort"
	"strconv"
	"strings"
)

var (
	allocated []MemoryRegion
)

type MemoryRegion struct {
	Start  uint64
	End    uint64
	Perms  string
	Offset uint64
	Device string
	Inode  uint64
}

// 解析 /proc/[pid]/maps
func parseProcMaps(pid int) ([]MemoryRegion, error) {
	data, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/maps", pid))
	if err != nil {
		return nil, err
	}

	var regions []MemoryRegion
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		// 解析地址范围
		addrs := strings.Split(fields[0], "-")
		start, _ := strconv.ParseUint(addrs[0], 16, 64)
		end, _ := strconv.ParseUint(addrs[1], 16, 64)

		region := MemoryRegion{
			Start:  start,
			End:    end,
			Perms:  fields[1],
			Offset: parseHex(fields[2]),
			Device: fields[3],
			Inode:  parseHex(fields[4]),
		}
		regions = append(regions, region)
	}
	return regions, nil
}

func findFreeMemory(pid int, minSize uint64) (uint64, error) {
	regions, err := parseProcMaps(pid)
	if err != nil {
		return 0, err
	}

	// 查找匿名可写区（权限为 rw-p）
	for _, r := range regions {
		if r.Perms == "rw-p" && r.Offset == 0 && r.Inode == 0 {
			if isRegionAllocated(allocated, r.Start, r.End) {
				continue
			}

			if addr, found := findSpaceInRegion(allocated, r.Start, r.End, minSize); found {
				// 记录分配
				allocated = append(allocated, MemoryRegion{
					Start: addr,
					End:   addr + minSize,
				})
				return addr, nil
			}
		}
	}
	return 0, fmt.Errorf("not found free memory")
}

func isRegionAllocated(regions []MemoryRegion, start, end uint64) bool {
	for _, r := range regions {
		// 检查是否有重叠
		if r.Start < end && r.End > start {
			return true
		}
	}
	return false
}

func findSpaceInRegion(allocated []MemoryRegion, regionStart, regionEnd, minSize uint64) (uint64, bool) {
	// 没有已分配区域，直接使用整个区域
	if len(allocated) == 0 {
		if regionEnd-regionStart >= minSize {
			return regionStart, true
		}
		return 0, false
	}

	// 按起始地址排序
	sort.Slice(allocated, func(i, j int) bool {
		return allocated[i].Start < allocated[j].Start
	})

	// 检查区域开头的空间
	if allocated[0].Start-regionStart >= minSize {
		return regionStart, true
	}

	// 检查分配区域之间的空间
	for i := 0; i < len(allocated)-1; i++ {
		gapStart := allocated[i].End
		gapEnd := allocated[i+1].Start
		if gapEnd-gapStart >= minSize {
			return gapStart, true
		}
	}

	// 检查区域末尾的空间
	last := allocated[len(allocated)-1]
	if regionEnd-last.End >= minSize {
		return last.End, true
	}

	return 0, false
}

func parseHex(s string) uint64 {
	if s == "0" {
		return 0
	}
	val, _ := strconv.ParseUint(s, 16, 64)
	return val
}
