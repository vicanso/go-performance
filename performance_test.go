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
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConcurrency(t *testing.T) {
	assert := assert.New(t)
	c := NewConcurrency()
	c.Inc()
	assert.Equal(int32(1), c.Current())
	assert.Equal(int64(1), c.Total())

	c.Dec()
	assert.Equal(int32(0), c.Current())
	assert.Equal(int64(1), c.Total())
}

func TestHttpServerConnStats(t *testing.T) {
	assert := assert.New(t)
	hs := NewHttpServerConnStats()
	hs.ConnState(nil, http.StateNew)
	hs.ConnState(nil, http.StateActive)
	stats := hs.Stats()
	assert.Equal(int32(1), stats.ConnProcessing)
	assert.Equal(int64(1), stats.ConnProcessedCount)
	assert.Equal(int32(1), stats.ConnAlive)
	assert.Equal(int64(1), stats.ConnCreatedCount)

	hs.ConnState(nil, http.StateIdle)
	hs.ConnState(nil, http.StateClosed)
	stats = hs.Stats()
	assert.Equal(int32(0), stats.ConnProcessing)
	assert.Equal(int64(1), stats.ConnProcessedCount)
	assert.Equal(int32(0), stats.ConnAlive)
	assert.Equal(int64(1), stats.ConnCreatedCount)
}

func TestCPUMemory(t *testing.T) {
	assert := assert.New(t)

	cpuMemory := CurrentCPUMemory()
	assert.NotEmpty(cpuMemory.GoMaxProcs)
	assert.NotEmpty(cpuMemory.ThreadCount)
	assert.NotEmpty(cpuMemory.MemSys)
}
