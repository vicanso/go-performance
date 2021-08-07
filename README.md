# go-performance

performance of application, include cpu, memory usage, and etc.


```go
// UpdateCPUUsage 需要定时调用更新CPU使用率
performance.UpdateCPUUsage(context.Background())
// 获取当前CPU与内存使用
performance.CurrentCPUMemory(context.Background())

httpServerConnStats := performance.NewHttpServerConnStats()
http.Server{
    ConnState: httpServerConnStats.ConnState
}

ioCounters, err := performance.IOCounters(context.Background())

connsStat, err := performance.Connections(context.Background())

ctxSwitchesStat, err := performance.NumCtxSwitches(context.Background())

count, err := performance.NumFds(context.Background())

pageFaultsStat, err := performance.PageFaults(context.Background())

openFilesStat, err := performance.OpenFiles(context.Background())
```