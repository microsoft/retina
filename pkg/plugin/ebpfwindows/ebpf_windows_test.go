// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
// nolint

package ebpfwindows

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os/exec"
	"testing"
	"time"
	"unsafe"

	"github.com/cilium/cilium/api/v1/flow"
	v1 "github.com/cilium/cilium/pkg/hubble/api/v1"
	kcfg "github.com/microsoft/retina/pkg/config"
	"github.com/microsoft/retina/pkg/controllers/cache"
	"github.com/microsoft/retina/pkg/enricher"
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/metrics"
	"github.com/microsoft/retina/pkg/pubsub"
	"go.uber.org/zap"
	"golang.org/x/sys/windows"
)

type FiveTuple struct {
	Proto   uint8
	SrcIP   uint32
	DstIP   uint32
	SrcPort uint16
	DstPort uint16
}

type Filter struct {
	Event   uint8
	SrcIP   uint32
	DstIP   uint32
	SrcPort uint16
	DstPort uint16
}

var (
	Event_WriterDLL = windows.NewLazyDLL("event_writer.dll")
)

func ParseIPToUInt(ipStr string) (uint32, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return 0, fmt.Errorf("ParseIPToUInt - invalid IP address")
	}

	ip = ip.To4()
	if ip == nil {
		return 0, fmt.Errorf("ParseIPToUInt- invalid IPV4 address")
	}
	return binary.BigEndian.Uint32(ip), nil
}

func GetRingData(l *log.ZapLogger, e *enricher.Enricher, ctx *context.Context, eventChannel chan int) {
	evReader := e.ExportReader()
	timeout := 180 * time.Second
	timeoutChan := time.After(timeout)
	getData := make(chan *v1.Event)
	check_five_tuple_exists := Event_WriterDLL.NewProc("check_five_tuple_exists")

	go func() {
		ev := evReader.NextFollow(*ctx)
		getData <- ev
	}()

	defer func() {
		err := evReader.Close()
		if err != nil {
			l.Error("Error closing the event reader", zap.Error(err))
		}
		l.Info("Enricher reader closed")
	}()

	select {
	case <-timeoutChan:
		l.Info("Timeout reached")
		eventChannel <- 1
		return
	case ev := <-getData:
		if ev == nil {
			l.Info("No more events, breaking loop")
			eventChannel <- 1
			return
		}

		switch ev.Event.(type) {
		case *flow.Flow:
			if flow := ev.GetFlow(); flow != nil {
				if ip := flow.GetIP(); ip != nil {
					if l4 := flow.GetL4(); l4 != nil {
						srcIP := ip.Source
						dstIP := ip.Destination
						srcIPU32, err := ParseIPToUInt(srcIP)
						if err != nil {
							l.Error("Error", zap.Error(err), zap.String("IP", srcIP))
							eventChannel <- 1
							return
						}
						dstIPU32, err := ParseIPToUInt(dstIP)
						if err != nil {
							l.Error("Error", zap.Error(err), zap.String("IP", dstIP))
							eventChannel <- 1
							return
						}
						if tcp := l4.GetTCP(); tcp != nil {
							srcPrt := uint16(tcp.GetSourcePort())
							dstPrt := uint16(tcp.GetDestinationPort())

							l.Info("TCP",
								zap.String("FlowType", flow.GetType().String()),
								zap.String("srcIP", srcIP),
								zap.String("dstIP", dstIP),
								zap.Uint16("srcP", srcPrt),
								zap.Uint16("dstP", dstPrt),
							)

							fvt := &FiveTuple{
								Proto:   6,
								SrcIP:   srcIPU32,
								DstIP:   dstIPU32,
								SrcPort: srcPrt,
								DstPort: dstPrt,
							}

							ret, _, _ := check_five_tuple_exists.Call(uintptr(unsafe.Pointer(fvt)))
							if ret == 0 {
								l.Info("Match found!")
								eventChannel <- 0
								return
							}
						}

						if udp := l4.GetUDP(); udp != nil {
							srcPrt := uint16(udp.GetSourcePort())
							dstPrt := uint16(udp.GetDestinationPort())

							l.Info("UDP",
								zap.String("FlowType", flow.GetType().String()),
								zap.String("srcIP", srcIP),
								zap.String("dstIP", dstIP),
								zap.Uint16("srcP", srcPrt),
								zap.Uint16("dstP", dstPrt),
							)

							fvt := &FiveTuple{
								Proto:   17,
								SrcIP:   srcIPU32,
								DstIP:   dstIPU32,
								SrcPort: srcPrt,
								DstPort: dstPrt,
							}
							ret, _, _ := check_five_tuple_exists.Call(uintptr(unsafe.Pointer(fvt)))
							if ret == 0 {
								l.Info("Match found!")
								eventChannel <- 0
								return
							}
						}
					}
				}
			}
		default:
			l.Info("Unknown event type", zap.Any("event", ev))
		}
	}

	l.Error("Could not find expected flow object")
	eventChannel <- 1
}

func GetAllInterfaces() ([]int, error) {
	interfaces, err := net.Interfaces()
	var ifaceList []int
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {
		ifaceList = append(ifaceList, iface.Index)
	}

	return ifaceList, nil
}

func SetupEventWriter(l *log.ZapLogger) (int, error) {
	if Event_WriterDLL == nil {
		return 1, fmt.Errorf("SetupEventWriter - cannot lookup Event_WriterDLL")
	}

	pin_maps_load_programs := Event_WriterDLL.NewProc("pin_maps_load_programs")
	if ret, _, err := pin_maps_load_programs.Call(); ret != 0 {
		return int(ret), fmt.Errorf("SetupEventWriter - %v", zap.Error(err))
	}

	attach_program_to_interface := Event_WriterDLL.NewProc("attach_program_to_interface")
	int_attach_count := 0
	if ifindexList, err := GetAllInterfaces(); err != nil {
		return 1, fmt.Errorf("SetupEventWriter - no interfaces found")
	} else {
		for _, ifidx := range ifindexList {
			//Continue when error
			if ret, _, err := attach_program_to_interface.Call(uintptr(ifidx)); ret != 0 {
				l.Error("SetupEventWriter - failed to attach event_writer", zap.Int("Interface", ifidx), zap.Error(err))
			} else {
				l.Info("Event_writer attached to interface", zap.Int("Ifindex", ifidx))
				int_attach_count += 1
			}
		}
	}

	return 0, nil
}

func CloseEventWriter() (int, error) {
	if Event_WriterDLL == nil {
		return 1, fmt.Errorf("CloseEventWriter - cannot lookup Event_WriterDLL")
	}

	unload_programs_detach := Event_WriterDLL.NewProc("unload_programs_detach")
	ret, _, err := unload_programs_detach.Call()
	if ret != 0 {
		return int(ret), fmt.Errorf("CloseEventWriter - %v", zap.Error(err))
	}
	return 0, nil
}

func Curl(url string) (int, error) {
	cmd := exec.Command("curl", url)
	_, err := cmd.Output()
	if err != nil {
		return 1, fmt.Errorf("Curl - %s", err)
	}

	return 0, nil
}

func TestMain(t *testing.T) {
	log.SetupZapLogger(log.GetDefaultLogOpts())
	l := log.Logger().Named("test-ebpf")
	ctx := context.Background()

	//Load and attach ebpf program
	if ret, err := SetupEventWriter(l); ret != 0 {
		l.Error("TestMain", zap.Error(err))
		t.Fail()
		return
	}

	defer func() {
		ret, err := CloseEventWriter()
		if ret != 0 {
			l.Error("TestMain", zap.Error(err))
			return
		}
		l.Info("Program successfully unloaded and detached")
	}()

	cfg := &kcfg.Config{
		MetricsInterval: 1 * time.Second,
		EnablePodLevel:  true,
	}

	c := cache.New(pubsub.New())
	e := enricher.New(ctx, c)
	e.Run()
	defer e.Reader.Close()

	metrics.InitializeMetrics()

	tt := New(cfg)
	err := tt.Stop()
	if err != nil {
		l.Error("TestMain - failed to stop plugin", zap.Error(err))
		return
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	err = tt.Generate(ctxTimeout)
	if err != nil {
		l.Error("TestMain - failed to generate plugin", zap.Error(err))
		return
	}

	err = tt.Compile(ctxTimeout)
	if err != nil {
		l.Error("TestMain - failed to compile plugin", zap.Error(err))
		return
	}

	err = tt.Init()
	if err != nil {
		l.Error("TestMain - failed to init plugin", zap.Error(err))
		return
	}

	go tt.Start(ctx)
	defer func() {
		err = tt.Stop()
		if err != nil {
			l.Error("TestMain - failed to stop plugin", zap.Error(err))
		}
	}()

	if ret, err := ValidateFlowObject(l, ctx, e, CiliumNotifyTrace); ret != 0 {
		l.Error("TestTraceNotify", zap.Error(err))
		t.Fail()
	}
	if ret, err := ValidateFlowObject(l, ctx, e, CiliumNotifyDrop); ret != 0 {
		l.Error("TestDropNotify", zap.Error(err))
		t.Fail()
	}
}

func ValidateFlowObject(l *log.ZapLogger, ctx context.Context, e *enricher.Enricher, evt_type uint8) (int, error) {
	eventChannel := make(chan int)
	set_filter := Event_WriterDLL.NewProc("set_filter")
	// Hardcoding IP addr for aka.ms - 23.213.38.151
	flt := &Filter{
		Event:   evt_type,
		SrcIP:   399845015,
		DstIP:   0,
		SrcPort: 0,
		DstPort: 0,
	}
	ret, _, err := set_filter.Call(uintptr(unsafe.Pointer(flt)))
	if ret != 0 {
		return int(ret), fmt.Errorf("ValidateFlowObject - %v", zap.Error(err))
	} else {
		l.Debug("ValidateFlowObject", zap.String("Filter", "Updated successfully"))
	}

	go GetRingData(l, e, &ctx, eventChannel)
	if ret, err := Curl("aka.ms"); ret != 0 {
		return ret, fmt.Errorf("ValidateFlowObject - %v", zap.Error(err))
	}
	result := <-eventChannel
	return result, nil
}
