// Copyright 2021 tree xie
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package performance

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	pnet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
	"go.uber.org/atomic"
)

// CPUMemory 应用CPU与Memory相关指标
type CPUMemory struct {
	GoMaxProcs   int   `json:"goMaxProcs"`
	ThreadCount  int32 `json:"threadCount"`
	RoutineCount int   `json:"routineCount"`

	CPUUser      int    `json:"cpuUser"`
	CPUSystem    int    `json:"cpuSystem"`
	CPUIdle      int    `json:"cpuIdle"`
	CPUNice      int    `json:"cpuNice"`
	CPUIowait    int    `json:"cpuIowait"`
	CPUIrq       int    `json:"cpuIrq"`
	CPUSoftirq   int    `json:"cpuSoftirq"`
	CPUSteal     int    `json:"cpuSteal"`
	CPUGuest     int    `json:"cpuGuest"`
	CPUGuestNice int    `json:"cpuGuestNice"`
	CPUUsage     int32  `json:"cpuUsage"`
	CPUBusy      string `json:"cpuBusy"`

	MemAlloc      int    `json:"memAlloc"`
	MemTotalAlloc int    `json:"memTotalAlloc"`
	MemSys        int    `json:"memSys"`
	MemLookups    uint64 `json:"memLookups"`
	MemMallocs    uint64 `json:"memMallocs"`
	MemFrees      uint64 `json:"memFrees"`

	MemHeapAlloc    int    `json:"memHeapAlloc"`
	MemHeapSys      int    `json:"memHeapSys"`
	MemHeapIdle     int    `json:"memHeapIdle"`
	MemHeapInuse    int    `json:"memHeapInuse"`
	MemHeapReleased int    `json:"memHeapReleased"`
	MemHeapObjects  uint64 `json:"memHeapObjects"`

	MemStackInuse int `json:"memStackInuse"`
	MemStackSys   int `json:"memStackSys"`

	MemMSpanInuse  int `json:"memMSpanInuse"`
	MemMSpanSys    int `json:"memMSpanSys"`
	MemMCacheInuse int `json:"memMCacheInuse"`
	MemMCacheSys   int `json:"memMCacheSys"`
	MemBuckHashSys int `json:"memBuckHashSys"`

	MemGCSys    int `json:"memGCSys"`
	MemOtherSys int `json:"memOtherSys"`

	LastGC        time.Time     `json:"lastGC"`
	NumGC         uint32        `json:"numGC"`
	NumForcedGC   uint32        `json:"numForcedGC"`
	RecentPause   string        `json:"recentPause"`
	RecentPauseNs time.Duration `json:"recentPauseNs"`
	PauseTotal    string        `json:"pauseTotal"`
	PauseTotalNs  time.Duration `json:"pauseTotalNs"`
	PauseNs       [256]uint64   `json:"pauseNs"`

	// 各指标收集耗时
	// 内存指标
	MemStatsTook time.Duration
	// 线程指标
	ThreadCountStatsTook time.Duration
	// CPU使用
	CPUTimeStatsTook time.Duration
}

type Performance struct {
	CPUMemory
	IOCountersStat     *IOCountersStat     `json:"ioCountersStat"`
	ConnStat           *ConnectionsCount   `json:"connStat"`
	NumCtxSwitchesStat *NumCtxSwitchesStat `json:"numCtxSwitchesStat"`
	PageFaultsStat     *PageFaultsStat     `json:"pageFaultsStat"`
	NumFdsStat         *NumFdsStat         `json:"numFdsStat"`
	OpenFilesStats     *OpenFilesStat      `json:"openFilesStats"`
}

type ConnectionsCount struct {
	Status     map[string]int `json:"status"`
	RemoteAddr map[string]int `json:"remoteAddr"`
	Count      int            `json:"count"`
	Took       time.Duration  `json:"took"`
}

// concurrency 记录并发量与总量
type concurrency struct {
	current atomic.Int32
	total   atomic.Int64
}
type (
	// httpServerConnStats http server conn的统计
	httpServerConnStats struct {
		aliveConcurrency   concurrency
		processConcurrency concurrency
	}
	// ConnStats conn stats
	ConnStats struct {
		ConnProcessing     int32 `json:"connProcessing"`
		ConnProcessedCount int64 `json:"connProcessedCount"`
		ConnAlive          int32 `json:"connAlive"`
		ConnCreatedCount   int64 `json:"connCreatedCount"`
	}
	cpuTimeStats struct {
		Time time.Time
		Last *cpu.TimesStat
		Took time.Duration
	}
)

func (perf *Performance) ToMap(prevPerf *Performance) map[string]interface{} {
	fields := map[string]interface{}{
		"goMaxProcs":   perf.GoMaxProcs,
		"threadCount":  perf.ThreadCount,
		"routineCount": perf.RoutineCount,
		// cpu相关
		"cpuUsage":     perf.CPUUsage,
		"cpuUser":      perf.CPUUser,
		"cpuSystem":    perf.CPUSystem,
		"cpuIdle":      perf.CPUIdle,
		"cpuNice":      perf.CPUNice,
		"cpuIowait":    perf.CPUIowait,
		"cpuIrq":       perf.CPUIrq,
		"cpuSoftirq":   perf.CPUSoftirq,
		"cpuSteal":     perf.CPUSteal,
		"cpuGuest":     perf.CPUGuest,
		"cpuGuestNice": perf.CPUGuestNice,
		// 内存使用相关
		"memAlloc":      perf.MemAlloc,
		"memTotalAlloc": perf.MemTotalAlloc,
		"memSys":        perf.MemSys,
		"memLookups":    perf.MemLookups,

		"memHeapAlloc":    perf.MemHeapAlloc,
		"memHeapSys":      perf.MemHeapSys,
		"memHeapIdle":     perf.MemHeapIdle,
		"memHeapInuse":    perf.MemHeapInuse,
		"memHeapReleased": perf.MemHeapReleased,
		"memHeapObjects":  perf.MemHeapObjects,
		"memStackInuse":   perf.MemStackInuse,
		"memStackSys":     perf.MemStackSys,
		"memMSpanInuse":   perf.MemMSpanInuse,
		"memMSpanSys":     perf.MemMSpanSys,
		"memMCacheInuse":  perf.MemMCacheInuse,
		"memMCacheSys":    perf.MemMCacheSys,
		"memBuckHashSys":  perf.MemBuckHashSys,
		"memGCSys":        perf.MemGCSys,
		"memOtherSys":     perf.MemOtherSys,
	}
	if prevPerf != nil {
		fields["memMallocs"] = int(perf.MemMallocs - prevPerf.MemMallocs)
		fields["memFrees"] = int(perf.MemFrees - prevPerf.MemFrees)
		fields["numGC"] = int(perf.NumGC - prevPerf.NumGC)
		fields["pauseMS"] = int((perf.PauseTotalNs - prevPerf.PauseTotalNs).Milliseconds())
	}
	mb := uint64(1024 * 1024)

	// io 相关统计
	if perf.IOCountersStat != nil &&
		prevPerf.IOCountersStat != nil {
		stat := perf.IOCountersStat
		prevStat := prevPerf.IOCountersStat
		readCount := stat.ReadCount - prevStat.ReadCount
		writeCount := stat.WriteCount - prevStat.WriteCount
		readBytes := stat.ReadBytes - prevStat.ReadBytes
		writeBytes := stat.WriteBytes - prevStat.WriteBytes

		fields["ioReadCount"] = readCount
		fields["ioWriteCount"] = writeCount
		fields["ioReadMBytes"] = readBytes / mb
		fields["ioWriteMBytes"] = writeBytes / mb
	}

	// 网络相关
	if perf.ConnStat != nil {
		for k, v := range perf.ConnStat.Status {
			// 如果该状态下对应的连接大于0，则记录此连接数
			if v > 0 {
				fields["connTotal"+k] = v
			}
		}
		fields["connTotal"] = perf.ConnStat.Count
	}
	// context切换相关
	if perf.NumCtxSwitchesStat != nil &&
		prevPerf.NumCtxSwitchesStat != nil {
		stat := perf.NumCtxSwitchesStat
		prevStat := prevPerf.NumCtxSwitchesStat

		fields["ctxSwitchesVoluntary"] = stat.Voluntary - prevStat.Voluntary
		fields["ctxSwitchesInvoluntary"] = stat.Involuntary - prevStat.Involuntary
	}
	// page fault相关
	if perf.PageFaultsStat != nil &&
		prevPerf.PageFaultsStat != nil {
		stat := perf.PageFaultsStat
		prevStat := prevPerf.PageFaultsStat
		fields["pageFaultMinor"] = stat.MinorFaults - prevStat.MinorFaults
		fields["pageFaultMajor"] = stat.MajorFaults - prevStat.MajorFaults
		fields["pageFaultChildMinor"] = stat.ChildMinorFaults - prevStat.ChildMinorFaults
		fields["pageFaultChildMajor"] = stat.ChildMajorFaults - prevStat.ChildMajorFaults
	}
	// fd 相关
	if perf.NumFdsStat != nil {
		fields["numFds"] = perf.NumFdsStat.Fds
	}

	return fields
}

// cpuUsage cpu使用率
var cpuUsage atomic.Int32
var currentProcess *process.Process
var lastCPUTimeStats *cpuTimeStats

func init() {
	p, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		panic(err)
	}
	currentProcess = p
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = UpdateCPUUsage(ctx)
}

// Inc inc
func (c *concurrency) Inc() int32 {
	c.total.Inc()
	return c.current.Inc()
}

// Dec dec
func (c *concurrency) Dec() int32 {
	return c.current.Dec()
}

// Current get current
func (c *concurrency) Current() int32 {
	return c.current.Load()
}

// Total get total
func (c *concurrency) Total() int64 {
	return c.total.Load()
}

// NewConcurrency create a new concurrency
func NewConcurrency() *concurrency {
	return &concurrency{}
}

// ConnState conn state change function
func (hs *httpServerConnStats) ConnState(c net.Conn, cs http.ConnState) {
	// 如果HTTP客户端调用时设置请求头Connection: close
	// 则状态为new->active->closed
	// 其它的是
	// new->active->idle
	// idle -> active
	// idle -> close
	switch cs {
	case http.StateNew:
		hs.aliveConcurrency.Inc()
	case http.StateActive:
		hs.processConcurrency.Inc()
	case http.StateIdle:
		hs.processConcurrency.Dec()
	case http.StateHijacked:
		fallthrough
	case http.StateClosed:
		hs.aliveConcurrency.Dec()
	}
}

// Stats get stats of http server conn
func (hs *httpServerConnStats) Stats() ConnStats {
	return ConnStats{
		ConnProcessing:     hs.processConcurrency.Current(),
		ConnProcessedCount: hs.processConcurrency.Total(),
		ConnAlive:          hs.aliveConcurrency.Current(),
		ConnCreatedCount:   hs.aliveConcurrency.Total(),
	}
}

// Dec processing request count
// 如果连接是强制关闭的(http请求头指定Connection: close)
// 则该请求不会触发StateIdle，需要单独调用此函数处理
func (hs *httpServerConnStats) DecProcessing() {
	hs.processConcurrency.Dec()
}

// NewHttpServerConnStats create a new http server conn stats
func NewHttpServerConnStats() *httpServerConnStats {
	return &httpServerConnStats{}
}

func countTotal(c *cpu.TimesStat) float64 {
	return c.User + c.System + c.Idle + c.Nice + c.Iowait + c.Irq +
		c.Softirq + c.Steal + c.Guest + c.GuestNice
}

func calculatePercent(t1, t2 *cpu.TimesStat, delta float64, numcpu int) float64 {
	if delta == 0 {
		return 0
	}

	delta_proc := countTotal(t2) - countTotal(t1)
	overall_percent := ((delta_proc / delta) * 100) * float64(numcpu)
	return overall_percent
}

// UpdateCPUUsage 更新cpu使用率
func UpdateCPUUsage(ctx context.Context) error {
	start := time.Now()
	cpuTimes, err := currentProcess.TimesWithContext(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	if lastCPUTimeStats != nil {
		numcpu := runtime.NumCPU()
		delta := (now.Sub(lastCPUTimeStats.Time).Seconds()) * float64(numcpu)
		usage := calculatePercent(lastCPUTimeStats.Last, cpuTimes, delta, numcpu)
		cpuUsage.Store(int32(usage))
	}
	lastCPUTimeStats = &cpuTimeStats{
		Time: now,
		Last: cpuTimes,
		Took: time.Since(start),
	}

	return nil
}

// CurrentCPUMemory 获取当前应用性能指标
func CurrentCPUMemory(ctx context.Context) CPUMemory {
	var mb uint64 = 1024 * 1024
	memStatsStart := time.Now()
	m := &runtime.MemStats{}
	runtime.ReadMemStats(m)
	memStatsTook := time.Since(memStatsStart)
	seconds := int64(m.LastGC) / int64(time.Second)
	size := uint32(len(m.PauseNs))
	index := (m.NumGC + size - 1) % size
	recentPauseNs := time.Duration(int64(m.PauseNs[index]))
	pauseTotalNs := time.Duration(int64(m.PauseTotalNs))
	var cpuTimes *cpu.TimesStat
	var cpuTimeStatsTook time.Duration
	if lastCPUTimeStats != nil {
		cpuTimes = lastCPUTimeStats.Last
		cpuTimeStatsTook = lastCPUTimeStats.Took
	}
	cpuBusy := ""
	if cpuTimes != nil {
		busy := time.Duration(int64(countTotal(cpuTimes)-cpuTimes.Idle)) * time.Second
		cpuBusy = busy.String()
	} else {
		cpuTimes = &cpu.TimesStat{}
	}
	threadCountStatsStart := time.Now()
	threadCount, _ := currentProcess.NumThreadsWithContext(ctx)
	threadCountStatsTook := time.Since(threadCountStatsStart)
	return CPUMemory{
		GoMaxProcs:   runtime.GOMAXPROCS(0),
		ThreadCount:  threadCount,
		RoutineCount: runtime.NumGoroutine(),

		CPUUsage:     cpuUsage.Load(),
		CPUBusy:      cpuBusy,
		CPUUser:      int(cpuTimes.User),
		CPUSystem:    int(cpuTimes.System),
		CPUIdle:      int(cpuTimes.Idle),
		CPUNice:      int(cpuTimes.Nice),
		CPUIowait:    int(cpuTimes.Iowait),
		CPUIrq:       int(cpuTimes.Irq),
		CPUSoftirq:   int(cpuTimes.Softirq),
		CPUSteal:     int(cpuTimes.Steal),
		CPUGuest:     int(cpuTimes.Guest),
		CPUGuestNice: int(cpuTimes.GuestNice),

		MemAlloc:      int(m.Alloc / mb),
		MemTotalAlloc: int(m.TotalAlloc / mb),
		MemSys:        int(m.Sys / mb),
		MemLookups:    m.Lookups,
		MemMallocs:    m.Mallocs,
		MemFrees:      m.Frees,

		MemHeapAlloc:    int(m.HeapAlloc / mb),
		MemHeapSys:      int(m.HeapSys / mb),
		MemHeapIdle:     int(m.HeapIdle / mb),
		MemHeapInuse:    int(m.HeapInuse / mb),
		MemHeapReleased: int(m.HeapReleased / mb),
		MemHeapObjects:  m.HeapObjects,

		MemStackInuse: int(m.StackInuse / mb),
		MemStackSys:   int(m.StackSys / mb),

		MemMSpanInuse:  int(m.StackInuse / mb),
		MemMSpanSys:    int(m.MSpanSys / mb),
		MemMCacheInuse: int(m.MCacheInuse / mb),
		MemMCacheSys:   int(m.MCacheSys / mb),
		MemBuckHashSys: int(m.BuckHashSys / mb),

		MemGCSys:    int(m.GCSys / mb),
		MemOtherSys: int(m.OtherSys / mb),

		LastGC:      time.Unix(seconds, 0),
		NumGC:       m.NumGC,
		NumForcedGC: m.NumForcedGC,

		RecentPause:   recentPauseNs.String(),
		RecentPauseNs: recentPauseNs,
		PauseTotal:    pauseTotalNs.String(),
		PauseTotalNs:  pauseTotalNs,
		PauseNs:       m.PauseNs,

		MemStatsTook:         memStatsTook,
		ThreadCountStatsTook: threadCountStatsTook,
		CPUTimeStatsTook:     cpuTimeStatsTook,
	}
}

type IOCountersStat struct {
	process.IOCountersStat
	Took time.Duration `json:"took"`
}

// IOCounters returns the conters stats info
func IOCounters(ctx context.Context) (*IOCountersStat, error) {
	start := time.Now()
	stat, err := currentProcess.IOCountersWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return &IOCountersStat{
		IOCountersStat: *stat,
		Took:           time.Since(start),
	}, nil
}

type ConnectionStat struct {
	Connections []pnet.ConnectionStat `json:"connections"`
	Took        time.Duration         `json:"took"`
}

// Connections returns the connections stats
func Connections(ctx context.Context) (*ConnectionStat, error) {
	start := time.Now()
	connections, err := currentProcess.ConnectionsWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return &ConnectionStat{
		Connections: connections,
		Took:        time.Since(start),
	}, nil
}

// ConnectionsStat return the count of connections stats
func ConnectionsStat(ctx context.Context) (*ConnectionsCount, error) {
	stats, err := Connections(ctx)
	if err != nil {
		return nil, err
	}
	count := ConnectionsCount{
		Status:     map[string]int{},
		RemoteAddr: map[string]int{},
		Took:       stats.Took,
	}
	for _, item := range stats.Connections {
		count.Count++
		if item.Status != "" {
			count.Status[item.Status] = count.Status[item.Status] + 1
		}
		addr := item.Raddr.IP
		if item.Raddr.Port != 0 {
			addr += fmt.Sprintf(":%d", item.Raddr.Port)
		}
		if addr != "" {
			count.RemoteAddr[addr] = count.RemoteAddr[addr] + 1
		}
	}
	return &count, nil
}

type NumCtxSwitchesStat struct {
	process.NumCtxSwitchesStat
	Took time.Duration `json:"took"`
}

// NumCtxSwitches returns the switch stats of process
func NumCtxSwitches(ctx context.Context) (*NumCtxSwitchesStat, error) {
	start := time.Now()
	stat, err := currentProcess.NumCtxSwitchesWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return &NumCtxSwitchesStat{
		NumCtxSwitchesStat: *stat,
		Took:               time.Since(start),
	}, nil
}

type NumFdsStat struct {
	Fds  int32         `json:"fds"`
	Took time.Duration `json:"took"`
}

// NumFds returns the count of fd
func NumFds(ctx context.Context) (*NumFdsStat, error) {
	start := time.Now()
	fds, err := currentProcess.NumFDsWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return &NumFdsStat{
		Fds:  fds,
		Took: time.Since(start),
	}, nil
}

type PageFaultsStat struct {
	process.PageFaultsStat
	Took time.Duration `json:"took"`
}

// PageFaults returns page fault stats
func PageFaults(ctx context.Context) (*PageFaultsStat, error) {
	start := time.Now()
	stat, err := currentProcess.PageFaultsWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return &PageFaultsStat{
		PageFaultsStat: *stat,
		Took:           time.Since(start),
	}, nil
}

type OpenFilesStat struct {
	OpenFiles []process.OpenFilesStat `json:"openFiles"`
	Took      time.Duration           `json:"took"`
}

// OpenFiles returns open file stats
func OpenFiles(ctx context.Context) (*OpenFilesStat, error) {
	start := time.Now()
	openFiles, err := currentProcess.OpenFilesWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return &OpenFilesStat{
		OpenFiles: openFiles,
		Took:      time.Since(start),
	}, nil
}

// GetPerformance 获取应用性能指标
func GetPerformance(ctx context.Context) *Performance {
	ioCountersStat, _ := IOCounters(ctx)
	connStat, _ := ConnectionsStat(ctx)
	numCtxSwitchesStat, _ := NumCtxSwitches(ctx)

	pageFaults, _ := PageFaults(ctx)
	openFilesStats, _ := OpenFiles(ctx)
	// fd 可以通过open files获取，减少一次查询
	var numFdsStat *NumFdsStat
	if openFilesStats != nil {
		numFdsStat = &NumFdsStat{
			Fds:  int32(len(openFilesStats.OpenFiles)),
			Took: openFilesStats.Took,
		}
	}
	return &Performance{
		CPUMemory:          CurrentCPUMemory(ctx),
		IOCountersStat:     ioCountersStat,
		ConnStat:           connStat,
		NumCtxSwitchesStat: numCtxSwitchesStat,
		NumFdsStat:         numFdsStat,
		PageFaultsStat:     pageFaults,
		OpenFilesStats:     openFilesStats,
	}
}
