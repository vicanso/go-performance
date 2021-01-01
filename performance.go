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
	"net"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"go.uber.org/atomic"
)

// CPUMemory 应用CPU与Memory相关指标
type CPUMemory struct {
	GoMaxProcs    int           `json:"goMaxProcs,omitempty"`
	ThreadCount   int32         `json:"threadCount,omitempty"`
	MemSys        int           `json:"memSys,omitempty"`
	MemHeapSys    int           `json:"memHeapSys,omitempty"`
	MemHeapInuse  int           `json:"memHeapInuse,omitempty"`
	MemFrees      uint64        `json:"memFrees,omitempty"`
	RoutineCount  int           `json:"routineCount,omitempty"`
	CPUUsage      int32         `json:"cpuUsage,omitempty"`
	LastGC        time.Time     `json:"lastGC,omitempty"`
	NumGC         uint32        `json:"numGC,omitempty"`
	RecentPause   string        `json:"recentPause,omitempty"`
	RecentPauseNs time.Duration `json:"recentPauseNs,omitempty"`
	PauseTotal    string        `json:"pauseTotal,omitempty"`
	PauseTotalNs  time.Duration `json:"pauseTotalNs,omitempty"`
	CPUBusy       string        `json:"cpuBusy,omitempty"`
	Uptime        string        `json:"uptime,omitempty"`
	PauseNs       [256]uint64   `json:"pauseNs,omitempty"`
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
		ConnProcessing     int32
		ConnProcessedCount int64
		ConnAlive          int32
		ConnCreatedCount   int64
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
func CurrentCPUMemory() CPUMemory {
	var mb uint64 = 1024 * 1024
	m := &runtime.MemStats{}
	runtime.ReadMemStats(m)
	seconds := int64(m.LastGC) / int64(time.Second)
	recentPauseNs := time.Duration(int64(m.PauseNs[(m.NumGC+255)%256]))
	pauseTotalNs := time.Duration(int64(m.PauseTotalNs))
	cpuTimes, _ := currentProcess.Times()
	cpuBusy := ""
	if cpuTimes != nil {
		busy := time.Duration(int64(cpuTimes.Total()-cpuTimes.Idle)) * time.Second
		cpuBusy = busy.String()
	}
	threadCount, _ := currentProcess.NumThreads()
	return CPUMemory{
		GoMaxProcs:    runtime.GOMAXPROCS(0),
		ThreadCount:   threadCount,
		MemSys:        int(m.Sys / mb),
		MemHeapSys:    int(m.HeapSys / mb),
		MemHeapInuse:  int(m.HeapInuse / mb),
		MemFrees:      m.Frees,
		RoutineCount:  runtime.NumGoroutine(),
		CPUUsage:      cpuUsage.Load(),
		LastGC:        time.Unix(seconds, 0),
		NumGC:         m.NumGC,
		RecentPause:   recentPauseNs.String(),
		RecentPauseNs: recentPauseNs,
		PauseTotal:    pauseTotalNs.String(),
		PauseTotalNs:  pauseTotalNs,
		CPUBusy:       cpuBusy,
		PauseNs:       m.PauseNs,
	}
}
