// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package filtermanager

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_newCache(t *testing.T) {
	f := getCache()
	assert.NotNil(t, f)

	f.data["1.1.1.1"] = requests{
		"test": map[RequestMetadata]bool{
			{RuleID: "test"}: true,
		},
	}

	f2 := getCache()
	assert.Equal(t, f, f2)
}

func Test_IPs(t *testing.T) {
	f := getCache()
	f.reset()

	ips := f.ips()
	assert.Equal(t, 0, len(ips))

	f.data["1.1.1.1"] = requests{}
	f.data["2.2.2.2"] = requests{}

	ips = f.ips()
	assert.Equal(t, 2, len(ips))

	delete(f.data, "1.1.1.1")
	ips = f.ips()
	assert.Equal(t, 1, len(ips))
}

func Test_reset(t *testing.T) {
	f := getCache()
	assert.NotNil(t, f)

	f.data["1.1.1.1"] = requests{}
	f.reset()
	assert.Equal(t, 0, len(f.data))
}

func Test_hasKey(t *testing.T) {
	f := getCache()
	f.data["1.1.1.1"] = requests{}
	assert.True(t, f.hasKey(net.ParseIP("1.1.1.1")))
	assert.False(t, f.hasKey(net.ParseIP("2.2.2.2")))
}

func addIPsHelper() {
	f := getCache()
	ip1 := net.ParseIP("1.1.1.1")
	ip2 := net.ParseIP("2.2.2.2")

	wg := sync.WaitGroup{}
	wg.Add(4)

	go func() {
		f.addIP(ip1, "trace1", RequestMetadata{RuleID: "task1"})
		wg.Done()
	}()
	go func() {
		f.addIP(ip1, "trace1", RequestMetadata{RuleID: "task2"})
		wg.Done()
	}()
	go func() {
		f.addIP(ip1, "trace2", RequestMetadata{RuleID: "task3"})
		wg.Done()
	}()
	go func() {
		f.addIP(ip2, "trace1", RequestMetadata{RuleID: "task1"})
		wg.Done()
	}()

	// Wait for goroutines to finish.
	wg.Wait()
}

func Test_addIP(t *testing.T) {
	addIPsHelper()

	f := getCache()
	assert.Equal(t, 2, len(f.data))

	expectedData := map[string]requests{
		"1.1.1.1": {
			"trace1": map[RequestMetadata]bool{
				{RuleID: "task1"}: true,
				{RuleID: "task2"}: true,
			},
			"trace2": map[RequestMetadata]bool{
				{RuleID: "task3"}: true,
			},
		},
		"2.2.2.2": {
			"trace1": map[RequestMetadata]bool{
				{RuleID: "task1"}: true,
			},
		},
	}
	assert.Equal(t, expectedData, f.data)

	// Add an IP that already exists.
	addIPsHelper()
	assert.Equal(t, expectedData, f.data)
}

func Test_deleteIP(t *testing.T) {
	addIPsHelper()

	f := getCache()
	assert.Equal(t, 2, len(f.data))

	ip1 := net.ParseIP("1.1.1.1")
	ip2 := net.ParseIP("2.2.2.2")
	ip3 := net.ParseIP("3.3.3.3")

	// Try deleting an IP that doesn't exist.
	res := f.deleteIP(ip3, "trace1", RequestMetadata{RuleID: "task1"})
	expectedData := map[string]requests{
		"1.1.1.1": {
			"trace1": map[RequestMetadata]bool{
				{RuleID: "task1"}: true,
				{RuleID: "task2"}: true,
			},
			"trace2": map[RequestMetadata]bool{
				{RuleID: "task3"}: true,
			},
		},
		"2.2.2.2": {
			"trace1": map[RequestMetadata]bool{
				{RuleID: "task1"}: true,
			},
		},
	}
	assert.False(t, res)
	assert.Equal(t, expectedData, f.data)

	res = f.deleteIP(ip1, "trace1", RequestMetadata{RuleID: "task1"})
	time.Sleep(10 * time.Millisecond)
	expectedData = map[string]requests{
		"1.1.1.1": {
			"trace1": map[RequestMetadata]bool{
				{RuleID: "task2"}: true,
			},
			"trace2": map[RequestMetadata]bool{
				{RuleID: "task3"}: true,
			},
		},
		"2.2.2.2": {
			"trace1": map[RequestMetadata]bool{
				{RuleID: "task1"}: true,
			},
		},
	}
	assert.False(t, res)
	assert.Equal(t, expectedData, f.data)

	res = f.deleteIP(ip2, "trace1", RequestMetadata{RuleID: "task1"})
	time.Sleep(1 * time.Millisecond)
	expectedData = map[string]requests{
		"1.1.1.1": {
			"trace1": map[RequestMetadata]bool{
				{RuleID: "task2"}: true,
			},
			"trace2": map[RequestMetadata]bool{
				{RuleID: "task3"}: true,
			},
		},
	}
	assert.True(t, res)
	assert.Equal(t, expectedData, f.data)

	res = f.deleteIP(ip1, "trace1", RequestMetadata{RuleID: "task2"})
	time.Sleep(1 * time.Millisecond)
	expectedData = map[string]requests{
		"1.1.1.1": {
			"trace2": map[RequestMetadata]bool{
				{RuleID: "task3"}: true,
			},
		},
	}
	assert.False(t, res)
	assert.Equal(t, expectedData, f.data)

	// Try deleting a task that doesn't exist.
	res = f.deleteIP(ip1, "trace2", RequestMetadata{RuleID: "task2"})
	time.Sleep(1 * time.Millisecond)
	expectedData = map[string]requests{
		"1.1.1.1": {
			"trace2": map[RequestMetadata]bool{
				{RuleID: "task3"}: true,
			},
		},
	}
	assert.False(t, res)
	assert.Equal(t, expectedData, f.data)

	res = f.deleteIP(ip1, "trace2", RequestMetadata{RuleID: "task3"})
	time.Sleep(1 * time.Millisecond)
	expectedData = map[string]requests{}
	assert.True(t, res)
	assert.Equal(t, expectedData, f.data)
}

func Test_multiOp(t *testing.T) {
	f := getCache()

	wg := sync.WaitGroup{}
	wg.Add(100)
	for i := 0; i < 100; i++ {
		ip := net.ParseIP(fmt.Sprintf("%d.%d.%d.%d", i, i, i, i))
		go func() {
			f.addIP(ip, Requestor(fmt.Sprintf("trace-%d", i)), RequestMetadata{RuleID: "task1"})
			wg.Done()
		}()
	}
	wg.Wait()
	assert.Equal(t, 100, len(f.data))

	fn := func(i int) {
		ip := net.ParseIP(fmt.Sprintf("%d.%d.%d.%d", i, i, i, i))
		res := f.deleteIP(ip, Requestor(fmt.Sprintf("trace-%d", i)), RequestMetadata{RuleID: "task1"})
		assert.True(t, res)
	}
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func() {
			i := i
			fn(i)
			wg.Done()
		}()
	}
	wg.Wait()
	assert.Equal(t, 0, len(f.data))
}
