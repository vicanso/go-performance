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
}

type ConnectionsCount struct {
	Status     map[string]int `json:"status"`
	RemoteAddr map[string]int `json:"remoteAddr"`
	Count      int            `json:"count"`
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
)

// cpuUsage cpu使用率
var cpuUsage atomic.Int32
var currentProcess *process.Process

func init() {
	p, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		panic(err)
	}
	currentProcess = p
	_ = UpdateCPUUsage()
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

// UpdateCPUUsage 更新cpu使用率
func UpdateCPUUsage() error {
	usage, err := currentProcess.Percent(0)
	if err != nil {
		return err
	}
	cpuUsage.Store(int32(usage))
	return nil
}

// CurrentCPUMemory 获取当前应用性能指标
func CurrentCPUMemory(ctx context.Context) CPUMemory {
	var mb uint64 = 1024 * 1024
	m := &runtime.MemStats{}
	runtime.ReadMemStats(m)
	seconds := int64(m.LastGC) / int64(time.Second)
	recentPauseNs := time.Duration(int64(m.PauseNs[(m.NumGC+255)%256]))
	pauseTotalNs := time.Duration(int64(m.PauseTotalNs))
	cpuTimes, _ := currentProcess.TimesWithContext(ctx)
	cpuBusy := ""
	if cpuTimes != nil {
		busy := time.Duration(int64(cpuTimes.Total()-cpuTimes.Idle)) * time.Second
		cpuBusy = busy.String()
	}
	threadCount, _ := currentProcess.NumThreadsWithContext(ctx)
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
	}
}

// IOCounters returns the conters stats info
func IOCounters(ctx context.Context) (*process.IOCountersStat, error) {
	return currentProcess.IOCountersWithContext(ctx)
}

// Connections returns the connections stats
func Connections(ctx context.Context) ([]pnet.ConnectionStat, error) {
	return currentProcess.ConnectionsWithContext(ctx)
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
	}
	for _, item := range stats {
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

// NumCtxSwitches returns the switch stats of process
func NumCtxSwitches(ctx context.Context) (*process.NumCtxSwitchesStat, error) {
	return currentProcess.NumCtxSwitchesWithContext(ctx)
}

// NumFds returns the count of fd
func NumFds(ctx context.Context) (int32, error) {
	return currentProcess.NumFDsWithContext(ctx)
}

// PageFaults returns page fault stats
func PageFaults(ctx context.Context) (*process.PageFaultsStat, error) {
	return currentProcess.PageFaultsWithContext(ctx)
}

// OpenFiles returns open file stats
func OpenFiles(ctx context.Context) ([]process.OpenFilesStat, error) {
	return currentProcess.OpenFilesWithContext(ctx)
}
