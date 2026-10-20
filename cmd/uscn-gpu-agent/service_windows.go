package main

import (
	"context"
	"fmt"
	"github.com/beijingcao/uscn-gpu-agent/internal/gpuagent"
	"syscall"
	"unsafe"
)

// Native SCM dispatcher; the binary is an actual Windows service, not a console
// executable falsely registered with sc.exe. Service control cancels the agent.
type serviceTable struct {
	Name  *uint16
	Entry uintptr
}
type serviceStatus struct{ ServiceType, CurrentState, ControlsAccepted, Win32ExitCode, ServiceSpecificExitCode, CheckPoint, WaitHint uint32 }

func runService(path string) error {
	adv := syscall.NewLazyDLL("advapi32.dll")
	set := adv.NewProc("SetServiceStatus")
	register := adv.NewProc("RegisterServiceCtrlHandlerExW")
	start := adv.NewProc("StartServiceCtrlDispatcherW")
	name, _ := syscall.UTF16PtrFromString("USCNGPUAgent")
	var result error
	mainCallback := syscall.NewCallback(func(argc uint32, argv uintptr) uintptr {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var handle uintptr
		update := func(state, controls, code uint32) {
			st := serviceStatus{ServiceType: 0x10, CurrentState: state, ControlsAccepted: controls, Win32ExitCode: code}
			_, _, _ = set.Call(handle, uintptr(unsafe.Pointer(&st)))
		}
		handler := syscall.NewCallback(func(control, event uint32, data, opaque uintptr) uintptr {
			if control == 1 || control == 5 {
				update(3, 0, 0)
				cancel()
			}
			return 0
		})
		handle, _, _ = register.Call(uintptr(unsafe.Pointer(name)), handler, 0)
		if handle == 0 {
			result = fmt.Errorf("Windows service registration unavailable")
			return 0
		}
		update(2, 0, 0)
		update(4, 1|4, 0)
		result = gpuagent.Run(ctx, path, gpuagent.Options{})
		code := uint32(0)
		if result != nil {
			code = 1
		}
		update(1, 0, code)
		return 0
	})
	table := [2]serviceTable{{name, mainCallback}, {nil, 0}}
	r, _, e := start.Call(uintptr(unsafe.Pointer(&table[0])))
	if r == 0 {
		return fmt.Errorf("Windows SCM dispatcher unavailable: %v", e)
	}
	return result
}
