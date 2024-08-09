package infiniband

import "testing/fstest"

var testFS = fstest.MapFS{ // nolint unused
	"infiniband/mlx5_ib0/ports/1/counters/excessive_buffer_overrun_errors": &fstest.MapFile{
		Data: []byte("1"),
	},
	"infiniband/mlx5_ib0/ports/1/counters/VL15_dropped": &fstest.MapFile{
		Data: []byte("1"),
	},
	"infiniband/mlx5_ib0/ports/2/counters/excessive_buffer_overrun_errors": &fstest.MapFile{
		Data: []byte("1"),
	},
	"infiniband/mlx5_ib0/ports/2/counters/VL15_dropped": &fstest.MapFile{
		Data: []byte("1"),
	},
	"infiniband/mlx5_an0/ports/1/counters/excessive_buffer_overrun_errors": &fstest.MapFile{
		Data: []byte("1"),
	},
	"infiniband/mlx5_an0/ports/1/counters/VL15_dropped": &fstest.MapFile{
		Data: []byte("1"),
	},
	"net/ib0/debug/link_down_reason": &fstest.MapFile{
		Data: []byte("1"),
	},
	"net/ib0/debug/lro_timeout": &fstest.MapFile{
		Data: []byte("1"),
	},
	"net/docker0/debug/link_down_reason": &fstest.MapFile{
		Data: []byte("1"),
	},
	"net/docker0/debug/lro_timeout": &fstest.MapFile{
		Data: []byte("1"),
	},
}
