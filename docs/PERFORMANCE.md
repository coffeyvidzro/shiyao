# Performance benchmarks

P2 benchmarks measure the latency and throughput of the VM execution path without making synthetic claims about host performance.

## Benchmarks

| Benchmark | Measures | Primary metric |
| --- | --- | --- |
| `BenchmarkPerformanceColdBoot` | Create, configure, and cold-start a Firecracker VM | `boot_us/op` |
| `BenchmarkPerformanceSnapshotResume` | Create, configure, and resume a prepared snapshot | `resume_us/op` |
| `BenchmarkPerformanceWarmPool` | Warm checkout → reset → check-in | `checkout_reset_checkin_us/op` |
| `BenchmarkPerformanceReset` | Guest reset operation only | `reset_us/op` |
| `BenchmarkPerformanceConcurrentProvisioning` | Parallel VM provisioning at concurrency 1, 2, 4, and configured maximum | `batch_us`, `provisions/sec` |

The cold-boot and snapshot measurements use the same `ProvisionVM` readiness boundary. A successful measurement means the VMM manager has reached `StateRunning`; it is not a claim that an arbitrary application inside the guest is ready.

## Running on a benchmark host

These are integration benchmarks and require Linux/KVM, Firecracker, a guest kernel/rootfs, the Shiyao guest agent, and a usable uplink interface.

```bash
sudo -E go test -tags=integration \
  -bench='BenchmarkPerformance' \
  -benchtime=1x \
  -count=5 \
  ./internal/vmm
```

Required environment:

```text
SHIYAO_BENCH_KERNEL=/path/to/vmlinux
SHIYAO_BENCH_ROOTFS=/path/to/rootfs.ext4
SHIYAO_BENCH_GUEST_AGENT=/path/to/shiyao-agent
SHIYAO_BENCH_UPLINK=eth0
```

Optional VM sizing:

```text
SHIYAO_BENCH_VCPU=2
SHIYAO_BENCH_MEMORY_MB=512
SHIYAO_BENCH_BOOT_ARGS=...
```

### Snapshot resume

Snapshot measurements require a prepared Firecracker snapshot compatible with the benchmark host and guest configuration:

```text
SHIYAO_BENCH_SNAPSHOT_MEM=/path/to/mem_file
SHIYAO_BENCH_SNAPSHOT_STATE=/path/to/vmstate_file
```

The snapshot must pass Shiyao's normal snapshot validation. Do not compare a snapshot built from a materially different guest image, CPU configuration, or memory size with the cold-boot fixture.

### Warm-pool reset

The default benchmark reset command is intentionally small and reproducible:

```text
rm -rf /tmp/shiyao-bench-reset && sync
```

A deployment-specific reset can be supplied with `SHIYAO_BENCH_RESET_COMMAND`. For meaningful security/performance comparisons, the reset command should represent the real production reset contract rather than a synthetic no-op.

## Methodology

For comparable results:

1. Use the same host, kernel, Firecracker version, guest kernel/rootfs, vCPU count, memory size, and network configuration across runs.
2. Pin or record CPU topology and avoid comparing idle and heavily loaded hosts as if they were equivalent.
3. Run cold boot and snapshot benchmarks independently; snapshot preparation time is not included in resume latency.
4. Report multiple repetitions (`-count`) and inspect the distribution rather than relying on one minimum or maximum.
5. Record the Firecracker version and guest fixture identifiers alongside benchmark output.
6. Keep cleanup outside the timed region where possible. The benchmark's primary latency is provisioning/readiness, not teardown.
7. For concurrency, compare both total batch latency and `provisions/sec`; a lower single-VM latency does not necessarily imply better aggregate throughput.
8. Warm-pool results should always be reported together with reset cost. A fast checkout that performs an expensive reset may not improve end-to-end request latency.

## Result template

Use this template when recording benchmark runs:

```text
Host:
CPU:
RAM:
Linux kernel:
KVM:
Firecracker:
Guest kernel:
Guest rootfs:
Guest agent:
vCPU / VM:
Memory / VM:
Network uplink:

Cold boot:
Snapshot resume:
Warm checkout + reset + check-in:
Reset:
Concurrent provisioning (1):
Concurrent provisioning (2):
Concurrent provisioning (4):
Concurrent provisioning (max):

Notes:
```

## Interpretation

These benchmarks are measurement infrastructure, not performance guarantees. Results depend strongly on the host kernel, KVM behavior, Firecracker release, storage, CPU contention, guest image, network setup, and snapshot characteristics.

The useful P2 question is not simply "how fast is Shiyao?" It is:

> Which lifecycle path gives the lowest end-to-end latency while preserving the isolation and reset guarantees established by P0?

That makes cold boot, snapshot resume, warm reset, and concurrent provisioning comparable parts of one lifecycle performance model.
