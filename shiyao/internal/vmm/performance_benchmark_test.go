//go:build integration

package vmm

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/coffeyvidzro/shiyao/internal/network"
	"github.com/coffeyvidzro/shiyao/internal/vsock"
)

// Real-host performance benchmarks. They deliberately require explicit
// Firecracker/KVM fixtures instead of silently benchmarking mocks.
//
// Required: SHIYAO_BENCH_KERNEL, SHIYAO_BENCH_ROOTFS,
// SHIYAO_BENCH_GUEST_AGENT, SHIYAO_BENCH_UPLINK.
// Snapshot: SHIYAO_BENCH_SNAPSHOT_MEM, SHIYAO_BENCH_SNAPSHOT_STATE.
// Optional: SHIYAO_BENCH_RESET_COMMAND.
//
// Example:
// sudo -E go test -tags=integration -bench='BenchmarkPerformance' -benchtime=1x ./internal/vmm

func BenchmarkPerformanceColdBoot(b *testing.B) {
	manager := benchmarkManager(b, false)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		ctx := context.Background()
		id := benchmarkVMID("cold", n)
		start := time.Now()
		inst, err := manager.ProvisionVM(ctx, id)
		if err != nil {
			b.Fatal(err)
		}
		b.ReportMetric(float64(time.Since(start).Microseconds()), "boot_us/op")
		b.StopTimer()
		if err := manager.DestroyVM(ctx, inst.ID); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
	}
}

func BenchmarkPerformanceSnapshotResume(b *testing.B) {
	manager := benchmarkManager(b, true)
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		ctx := context.Background()
		id := benchmarkVMID("snapshot", n)
		start := time.Now()
		inst, err := manager.ProvisionVM(ctx, id)
		if err != nil {
			b.Fatal(err)
		}
		b.ReportMetric(float64(time.Since(start).Microseconds()), "resume_us/op")
		b.StopTimer()
		if err := manager.DestroyVM(ctx, inst.ID); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
	}
}

func BenchmarkPerformanceWarmPool(b *testing.B) {
	manager := benchmarkManager(b, false)
	ctx := context.Background()
	inst, err := manager.ProvisionVM(ctx, benchmarkVMID("warm-seed", 0))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = manager.DestroyVM(ctx, inst.ID) })
	pool, err := NewWarmPool(1)
	if err != nil {
		b.Fatal(err)
	}
	if err := pool.Add(inst); err != nil {
		b.Fatal(err)
	}

	reset := benchmarkReset()
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		lease := benchmarkVMID("lease", n)
		start := time.Now()
		got, err := pool.Checkout(lease)
		if err != nil {
			b.Fatal(err)
		}
		if err := pool.Checkin(ctx, lease, reset); err != nil {
			b.Fatal(err)
		}
		b.ReportMetric(float64(time.Since(start).Microseconds()), "checkout_reset_checkin_us/op")
		if got != inst {
			b.Fatal("warm pool returned unexpected instance")
		}
	}
}

func BenchmarkPerformanceReset(b *testing.B) {
	manager := benchmarkManager(b, false)
	ctx := context.Background()
	inst, err := manager.ProvisionVM(ctx, benchmarkVMID("reset-seed", 0))
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = manager.DestroyVM(ctx, inst.ID) }()
	pool, err := NewWarmPool(1)
	if err != nil {
		b.Fatal(err)
	}
	if err := pool.Add(inst); err != nil {
		b.Fatal(err)
	}
	reset := benchmarkReset()

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		lease := benchmarkVMID("reset", n)
		got, err := pool.Checkout(lease)
		if err != nil {
			b.Fatal(err)
		}
		start := time.Now()
		if err := reset(ctx, got); err != nil {
			b.Fatal(err)
		}
		b.ReportMetric(float64(time.Since(start).Microseconds()), "reset_us/op")
		if err := pool.Checkin(ctx, lease, func(context.Context, *Instance) error { return nil }); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPerformanceConcurrentProvisioning(b *testing.B) {
	manager := benchmarkManager(b, false)
	max := DefaultManagerLimits().MaxConcurrentProvision
	for _, width := range []int{1, 2, 4, max} {
		b.Run(fmt.Sprintf("concurrency-%d", width), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for batch := 0; batch < b.N; batch++ {
				ctx := context.Background()
				ids := make([]string, width)
				instances := make([]*Instance, width)
				var wg sync.WaitGroup
				errCh := make(chan error, width)
				startGate := make(chan struct{})
				for i := range ids {
					ids[i] = benchmarkVMID(fmt.Sprintf("parallel-%d", width), batch*width+i)
					wg.Add(1)
					go func(i int) {
						defer wg.Done()
						<-startGate
						inst, err := manager.ProvisionVM(ctx, ids[i])
						if err != nil {
							errCh <- err
							return
						}
						instances[i] = inst
					}(i)
				}
				start := time.Now()
				close(startGate)
				wg.Wait()
				close(errCh)
				elapsed := time.Since(start)
				for err := range errCh {
					b.Fatal(err)
				}
				b.ReportMetric(float64(elapsed.Microseconds()), "batch_us")
				b.ReportMetric(float64(width)/elapsed.Seconds(), "provisions/sec")
				b.StopTimer()
				for _, inst := range instances {
					if inst != nil {
						if err := manager.DestroyVM(ctx, inst.ID); err != nil {
							b.Fatal(err)
						}
					}
				}
				b.StartTimer()
			}
		})
	}
}

func benchmarkManager(b *testing.B, snapshot bool) *Manager {
	b.Helper()
	kernel, rootfs, guestAgent, uplink := os.Getenv("SHIYAO_BENCH_KERNEL"), os.Getenv("SHIYAO_BENCH_ROOTFS"), os.Getenv("SHIYAO_BENCH_GUEST_AGENT"), os.Getenv("SHIYAO_BENCH_UPLINK")
	if kernel == "" || rootfs == "" || guestAgent == "" || uplink == "" {
		b.Skip("set SHIYAO_BENCH_KERNEL, SHIYAO_BENCH_ROOTFS, SHIYAO_BENCH_GUEST_AGENT and SHIYAO_BENCH_UPLINK")
	}
	cfg := Config{KernelPath: kernel, RootfsPath: rootfs, VCPUCount: benchmarkInt("SHIYAO_BENCH_VCPU", 2), MemSizeMB: benchmarkInt("SHIYAO_BENCH_MEMORY_MB", 512), BootArgs: os.Getenv("SHIYAO_BENCH_BOOT_ARGS"), GuestAgentPath: guestAgent}
	netCfg := network.DefaultConfig(fmt.Sprintf("shy%d", time.Now().UnixNano()%10000))
	netCfg.UplinkInterface = uplink
	snapCfg := SnapshotConfig{EnableResume: snapshot}
	if snapshot {
		snapCfg.MemFilePath, snapCfg.StateFilePath = os.Getenv("SHIYAO_BENCH_SNAPSHOT_MEM"), os.Getenv("SHIYAO_BENCH_SNAPSHOT_STATE")
		if snapCfg.MemFilePath == "" || snapCfg.StateFilePath == "" {
			b.Skip("set snapshot fixture paths")
		}
	}
	return NewManager(cfg, netCfg, vsock.Config{}, snapCfg)
}

func benchmarkReset() func(context.Context, *Instance) error {
	command := os.Getenv("SHIYAO_BENCH_RESET_COMMAND")
	if command == "" {
		command = "rm -rf /tmp/shiyao-bench-reset && sync"
	}
	return func(ctx context.Context, inst *Instance) error {
		req := vsock.ExecRequest{Version: vsock.ProtocolVersion, ID: benchmarkVMID("reset-request", int(time.Now().UnixNano())), Command: "/bin/sh", Args: []string{"-c", command}, TimeoutMS: 30_000}
		_, err := vsock.Exec(ctx, inst.SocketPath, req)
		return err
	}
}

func benchmarkVMID(prefix string, n int) string { return fmt.Sprintf("bench-%s-%d", prefix, n) }
func benchmarkInt(name string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(name))
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
