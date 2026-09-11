// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	config "github.com/linuxdeepin/lastore-daemon/src/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLsblkOutputInvalidJSON(t *testing.T) {
	_, err := parseLsblkOutput([]byte("not json"))
	assert.Error(t, err)
}

func TestParseLsblkOutput(t *testing.T) {
	gb := int64(1024 * 1024 * 1024)
	// Crafted lsblk JSON exercising every branch of the parsing loop.
	devices := BlockDevices{
		Blockdevices: []BlockDevice{
			// removable disk is skipped
			{Name: "sda", Removable: true, Type: "disk", Size: 500 * gb},
			// non-disk entry is skipped
			{Name: "sdb1", Type: "part", Size: 100 * gb},
			// disk with no children and no fsuse -> free == size
			{Name: "sdc", Type: "disk", Size: 256 * gb},
			// disk with no children and fsuse parse error -> free == size
			{Name: "sdd", Type: "disk", Size: 512 * gb, Fsuse: "bad%"},
			// disk with no children and valid fsuse
			{Name: "sde", Type: "disk", Size: 1000 * gb, Fsuse: "50%"},
			// disk with children (mixed fsuse / no fsuse / parse error)
			{Name: "sdf", Type: "disk", Size: 2000 * gb, Children: []Partition{
				{Name: "sdf1", Size: 100 * gb, Fsuse: "10%"},
				{Name: "sdf2", Size: 200 * gb},
				{Name: "sdf3", Size: 300 * gb, Fsuse: "oops%"},
			}},
			// disk with children whose free capacity sums to zero -> free == size
			{Name: "sdg", Type: "disk", Size: 3000 * gb, Children: []Partition{
				{Name: "sdg1", Size: 100 * gb, Fsuse: "100%"},
			}},
		},
	}
	out, err := json.Marshal(devices)
	require.NoError(t, err)

	infos, err := parseLsblkOutput(out)
	require.NoError(t, err)
	require.Len(t, infos, 5)

	assert.Equal(t, "sdc", infos[0].DiskNo)
	assert.Equal(t, int64(512*gb), infos[0].TotalCapacity)
	assert.Equal(t, int64(256*gb), infos[0].FreeCapacity)

	assert.Equal(t, "sdd", infos[1].DiskNo)
	assert.Equal(t, int64(1024*gb), infos[1].TotalCapacity)
	assert.Equal(t, int64(512*gb), infos[1].FreeCapacity)

	assert.Equal(t, "sde", infos[2].DiskNo)
	assert.Equal(t, int64(1024*gb), infos[2].TotalCapacity)
	assert.Equal(t, int64(500*gb), infos[2].FreeCapacity)

	// sdf: 100*(100-10)/100 + 200 + skip(sdf3) = 90 + 200 = 290 GB
	assert.Equal(t, "sdf", infos[3].DiskNo)
	assert.Equal(t, int64(2048*gb), infos[3].TotalCapacity)
	assert.Equal(t, int64(290*gb), infos[3].FreeCapacity)

	// sdg: child free = 100*(100-100)/100 = 0 -> fall back to disk size
	assert.Equal(t, "sdg", infos[4].DiskNo)
	assert.Equal(t, int64(4096*gb), infos[4].TotalCapacity)
	assert.Equal(t, int64(3000*gb), infos[4].FreeCapacity)
}

func TestPostHardwareInfoDiskError(t *testing.T) {
	oldDisk, oldMem := getDiskSizeFn, getMemorySizeByDmiFn
	t.Cleanup(func() { getDiskSizeFn = oldDisk; getMemorySizeByDmiFn = oldMem })
	getDiskSizeFn = func() ([]DiskInfo, error) { return nil, errors.New("no lsblk") }

	err := postHardwareInfo(&config.Config{})
	assert.EqualError(t, err, "cannot get disk infos: no lsblk")
}

func TestPostHardwareInfoMemoryError(t *testing.T) {
	oldDisk, oldMem := getDiskSizeFn, getMemorySizeByDmiFn
	t.Cleanup(func() { getDiskSizeFn = oldDisk; getMemorySizeByDmiFn = oldMem })
	getDiskSizeFn = func() ([]DiskInfo, error) {
		return []DiskInfo{{DiskNo: "sda", TotalCapacity: 1024, FreeCapacity: 512}}, nil
	}
	getMemorySizeByDmiFn = func() ([]PostMemoryInfo, error) { return nil, errors.New("no dmidecode") }

	err := postHardwareInfo(&config.Config{})
	assert.EqualError(t, err, "cannot get memory infos: no dmidecode")
}

// newPostHardwareInfoServer returns an httptest server plus the config that
// points postHardwareInfo at it, with hardware probing stubbed out.
func newPostHardwareInfoServer(t *testing.T, handler http.HandlerFunc) *config.Config {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	oldDisk, oldMem := getDiskSizeFn, getMemorySizeByDmiFn
	t.Cleanup(func() { getDiskSizeFn = oldDisk; getMemorySizeByDmiFn = oldMem })
	getDiskSizeFn = func() ([]DiskInfo, error) {
		return []DiskInfo{{DiskNo: "sda", TotalCapacity: 1024, FreeCapacity: 512}}, nil
	}
	getMemorySizeByDmiFn = func() ([]PostMemoryInfo, error) {
		return []PostMemoryInfo{{MemoryNo: "1", Capacity: 8192}}, nil
	}
	return &config.Config{PlatformUrl: server.URL}
}

func TestPostHardwareInfoSuccess(t *testing.T) {
	t.Setenv("DBUS_SYSTEM_BUS_ADDRESS", "unix:path=/tmp/lastore-nonexistent-dbus-socket")
	var gotBody []byte
	c := newPostHardwareInfoServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/terminal/hardware", r.URL.Path)
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		gotBody = append(gotBody, buf[:n]...)
		w.WriteHeader(http.StatusOK)
	})

	err := postHardwareInfo(c)
	assert.NoError(t, err)
	assert.NotEmpty(t, gotBody)
}

func TestPostHardwareInfoServerError(t *testing.T) {
	t.Setenv("DBUS_SYSTEM_BUS_ADDRESS", "unix:path=/tmp/lastore-nonexistent-dbus-socket")
	c := newPostHardwareInfoServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	err := postHardwareInfo(c)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestPostHardwareInfoNetworkError(t *testing.T) {
	t.Setenv("DBUS_SYSTEM_BUS_ADDRESS", "unix:path=/tmp/lastore-nonexistent-dbus-socket")
	oldDisk, oldMem := getDiskSizeFn, getMemorySizeByDmiFn
	t.Cleanup(func() { getDiskSizeFn = oldDisk; getMemorySizeByDmiFn = oldMem })
	getDiskSizeFn = func() ([]DiskInfo, error) {
		return []DiskInfo{{DiskNo: "sda", TotalCapacity: 1024, FreeCapacity: 512}}, nil
	}
	getMemorySizeByDmiFn = func() ([]PostMemoryInfo, error) {
		return []PostMemoryInfo{{MemoryNo: "1", Capacity: 8192}}, nil
	}
	// Unroutable URL: the POST fails fast without real network.
	err := postHardwareInfo(&config.Config{PlatformUrl: "http://127.0.0.1:0"})
	assert.Error(t, err)
}
