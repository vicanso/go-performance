# go-performance

performance of application, include cpu, memory usage, and etc.


```go
// UpdateCPUUsage 需要定时调用更新CPU使用率
performance.UpdateCPUUsage()
// 获取当前CPU与内存使用
performance.CurrentCPUMemory()

httpServerConnStats := performance.NewHttpServerConnStats()
http.Server{
    ConnState: httpServerConnStats.ConnState
}
```