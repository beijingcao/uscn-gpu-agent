package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/beijingcao/uscn-gpu-agent/internal/gpuagent"
	"github.com/beijingcao/uscn-gpu-agent/internal/hardwareinventory"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	if e := mainRun(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func defaultConfig() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "uscn-gpu-agent", "device.json")
}
func mainRun() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("USCN GPU Agent %s: inspect | enroll | run | service | version", gpuagent.Version)
	}
	action := os.Args[1]
	fs := flag.NewFlagSet(action, flag.ContinueOnError)
	path := fs.String("config", defaultConfig(), "protected config outside source checkout")
	platform := fs.String("platform", "https://uscn.si", "USCN platform URL")
	name := fs.String("name", "GPU server", "device display name")
	if e := fs.Parse(os.Args[2:]); e != nil {
		return e
	}
	switch action {
	case "version":
		fmt.Println(gpuagent.Version)
		return nil
	case "inspect":
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
		defer cancel()
		v, e := hardwareinventory.Collect(ctx)
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(v)
	case "enroll":
		fmt.Fprintln(os.Stderr, "请输入上架页面生成的一次性设备配对码（不会写入日志或命令历史）：")
		code, e := bufio.NewReader(os.Stdin).ReadString('\n')
		if e != nil {
			return fmt.Errorf("pairing code required on standard input")
		}
		code = strings.TrimSpace(code)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		v, e := gpuagent.Enroll(ctx, *path, *platform, code, *name)
		if e != nil {
			return e
		}
		fmt.Println("USCN GPU device paired:", v.DeviceID)
		return nil
	case "run":
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		return gpuagent.Run(ctx, *path, gpuagent.Options{Status: func(s string) { fmt.Fprintln(os.Stderr, s) }})
	case "service":
		return runService(*path)
	default:
		return fmt.Errorf("unknown GPU agent command")
	}
}
