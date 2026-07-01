# Changelog

## Unreleased

- fix: guard runtime crash paths — no nil-deref reading a process exit code when a runtime binary fails to start, no index panic parsing empty runtime output (`ociruntime`), no divide-by-zero when a netfault attack has no interfaces, and no nil-deref when a stress/memfill/diskfill process wrapper's `Exited`/`Stop` is called without a successful `Start`
- fix: reject newline injection into `tc`/`iptables` batch stdin and into `dig` stdin, so an unsanitized interface name / rate / hostname forwarded from user input can no longer inject an extra privileged command; also capture the process pid before the SIGTERM timer goroutine to avoid a data race, and skip un-parseable IPs in DNS resolution

## 1.10.0

- **Breaking:** netfault snapshot now lives in caller-owned state, not a process-local map.
  `netfault.Apply` returns `(QdiscSnapshot, error)`; `netfault.Revert` takes the snapshot as a
  fourth argument. Types `QdiscSnapshot` and `InterfaceSnapshot` are exported and JSON-serializable
  so callers can persist them in their action's per-execution state. The in-memory `snapshotStore`
  is removed. This makes the snapshot survive extension restarts mid-attack (state lives on the
  Steadybit platform, not the extension pod) and removes the cross-attack store coordination that
  was already redundant given `activeNetfault` serializes attacks per netns. `tc.Object` JSON
  roundtrip is verified by a new regression test (`TestQdiscSnapshotJSONRoundtrip`) covering mq,
  tuned fq (BucketsLog/Horizon), prio, htb, clsact, ingress, pfifo_fast, noqueue, and filters.

## 1.9.0

- **Breaking:** `netfault.SetSnapshotRestore` is removed. The snapshot/restore path now runs whenever strict mode is OFF (i.e. `SetStrictRootQdisc(false)`), and is implicitly disabled when strict mode is ON because preflight already refuses non-`noqueue` roots. Callers should drop their `SetSnapshotRestore(...)` call; the behaviour is now driven entirely by `SetStrictRootQdisc`.

## 1.8.2

- netfault: opt-in qdisc snapshot/restore. With `SetSnapshotRestore(true)` (env var `STEADYBIT_EXTENSION_NETWORK_SNAPSHOT_RESTORE=true`), Apply captures the root qdisc tree (qdiscs + filters, incl. tuned `fq` params) of every target interface via RTNETLINK and Revert replays it after the attack's `tc del`. Preserves cloud-tuned roots (e.g. GKE's `mq + fq` with `buckets=32768 horizon=2s`) that previously reset to kernel defaults and degraded the host until reboot. Off by default; Linux only.
- netfault snapshot/restore: claim each saved auto-managed root handle via raw RTNETLINK before re-applying children (the kernel hides the auto-attached mq/clsact/ingress handle, so saved Parent refs would otherwise fail with ENOENT); re-anchor child Parents onto the live root as a defensive fallback; strip Stats/XStats/Stats2 from objects before Replace (go-tc rejects them); topological-sort qdiscs parent-first for N-level trees; classify `noqueue`/`pfifo_fast` as kernel-auto-managed.
- netfault snapshot/restore: lifecycle correctness — drop the snapshot if Apply errors, retain it on Revert failure (for retry), Filter().Replace() instead of Add() so leftover attack filters get overwritten, fail-fast on filter-read errors instead of storing partial snapshots.
- netfault snapshot/restore: debug-friendly logging. Apply logs the captured snapshot at INFO in `tc qdisc show`-style format; Revert re-snapshots after restore and logs INFO when state matches, WARN with a diff when it doesn't.

## 1.8.1

- stress: fix `ReadCpusAllowedCount` to count CPUs across all words of the
  `Cpus_allowed` mask. On hosts with more than 32 CPUs the kernel prints the
  mask as comma-separated 32-bit hex words, so the previous single-word parse
  capped the detected CPU count at 32 (causing stress-cpu with "all cores" to
  use only 32 workers).

## 1.8.0

- netfault: use `tc qdisc replace` (instead of `add`) for the root qdisc in
  delay/loss/corruption/bandwidth attacks so they no longer fail on hosts
  with a pre-existing root qdisc (e.g. `mq` on GKE COS / EKS / AKS).
- netfault: add preflight check that warns when an interface has a
  user-installed root qdisc (anything other than `mq`, `noqueue`,
  `pfifo_fast`, `fq_codel`, `fq`); the kernel default will be restored
  after revert in that case.
- **Breaking:** `netfault.Apply` now returns `([]string, error)` — the
  string slice contains preflight warnings to surface to the user.
- **Breaking:** The `Opts` interface no longer requires `ipCommands` or
  `tcCommands`. Subsystem behavior is now opt-in via two new optional
  interfaces: `tcCommandProvider` (`tcCommands` + `tcRootQdiscInterfaces`)
  and `ipCommandProvider` (`ipCommands`), mirroring the existing
  `iptablesScriptProvider`. External `Opts` implementations that returned
  `nil, nil` from these methods can simply remove them; external callers
  that consumed those methods need a type assertion first.

## 1.7.0

- Adds Hostnames []string to Opts, forwarded as repeated --hostname args to the
  dns-inject binary (>= v0.2.0). The new HostnameFiltered counter is parsed from
  the metrics JSON so callers can show how many DNS queries were skipped because
  their qname did not match the configured hostnames.

## 1.6.1

- Add UseMangleChain to TcpResetOpts to enable tcp reset on istio

## 1.6.0

- Add dns-inject wrapper
- Add tcp_reset network fault
- Move network faults using tc/ip/iptables to netfault package

## 1.5.15

- Add filldisk helpers to validate target directory

## 1.5.14

- Set OOM score adjustment in disc fill command

## 1.5.13

- Rephrase conflicting network attack message

## 1.5.12

- Add more context when network attack is conflicting with active one

## 1.5.11

- Update dependencies

## 1.5.10

## 1.5.9

- fix: failing to read correct cgroupV1 path when mixed with v2

## 1.5.7

- fix: correctly reference named network namespace in ip netns exec calls

## 1.5.6

 - fix: ensure stable sortet includes/excludes for network attacks

## 1.5.5

- fix: make DNS hostname resoultion lenient towards case changed
- refa: add STEADYBIT_EXTENSION_DIG_TIMEOUT to configure DNS hostname resolution timeout
- chore: add full trace logging for the dig output and extend the error messages

## 1.5.2

- fix: don't use crun on cgroup v1 systems

## 1.5.1

- refa: rename STEADYBIT_EXTENSION_OCIRUNTIME_RUNTIME_PATH to STEADYBIT_EXTENSION_OCIRUNTIME_PATH

## 1.5.0

- refa: rename runc package to ociruntime package
- feat: add support to run steadybit sidecar containers with crun

## 1.4.1

- feat: add option to ignore cgroup to fill memory

## 1.4.0

- feat: support attacks without using runc

## 1.3.2

- fix: propagate extension environment to started runc processes

## 1.3.1

- perf: allow which namespaces to inspect for a process

## 1.3.0

- Update dependencies (golang 1.24)

## 1.2.24

- Remove dependency on lsns

## 1.2.23

- refa: improve error handling in disk fill

## 1.2.22

- fix: use CAP_DAC_OVERRIDE in stress io attacks to ignore file permissions

## 1.2.21

- fix: use CAP_DAC_OVERRIDE in fill disk attacks to ignore file permissions

## 1.2.20

- fix: properly check file not exist errors during namespace lookup

## 1.2.19

- fix: named network namespace lookup with unexpected name format

## 1.2.18

- feat: add utils.ReadCpusAllowedCount

## 1.2.16

- chore: improve log handling in case of "orphaned" named network namespaces

## 1.2.15

- fix: properly rollback network attacks in case an "orphaned" named network namespace are present

## 1.2.14

- feat: make cap_sys_resource optional for resource attacks

## 1.2.13

- feat: add configuration option to change namespace listing method

## 1.2.12

- refa: add network.NoopOpts for testing purposes

## 1.2.11

- fix: executeIpCommands, split stdout and stderr

## 1.2.10

- feat: add helper to detect cilium

## 1.2.9

- fix: Try aggregating huge list of excludes for network attacks to reduce tc rules needed

## 1.2.8

- refa: Fail early if we know, that we generated too many tc commands
- refa: Only use these exclusion rules which are deemed necessary for the includes

## 1.2.7

- refa: use nsenter instead of runc for running fill memory attacks

## 1.2.6

- fix: check ip command family inet6 support

## 1.2.4

- feat: add common code for fill memory attacks

## 1.2.3

- feat: allow comments on include/excludes
- fix: don't use prio priomap defaults

## 1.2.2

- fix: Race condition in network attacks reporting attack for namespace still active, when it isn't

## 1.2.1

- feat: run stress attacks when cgroup fs was mounted with nsdelegate option

## 1.2.0

- Add ParseCIDRs to parse ip Addresses and CIDRs for network attacks.
- Resolve will not accept empty strings or ip addresses anymore.
- IpToNet was renamed to IpsToNets

## 1.1.12

- Check when CGroup2 nsdelegate is used and running containers on other CGroups

## 1.1.10

- Added noop mode for diskfill attack to avoid errors when the disk is already full enough

## 1.1.8

- Added hint if kernel modules are missing for tc

## 1.1.7

- Prevent different network attacks on the same network ns

## 1.1.6

- Workaround for failing lsns

## 1.1.5

- avoid collisions for cgroups used by stress-ng attacks

## 1.1.4

- Don't apply ipv6 rules if kernel module was disabled

## 1.1.3

- Read cgroup path of container using the root cgroup

## 1.1.2

- fix wrong cgroup path for stress

## 1.1.1

- more robust diskusage helper

## 1.1.0

- add runc helper for running containers
- add stress-ng, diskfill and network helpers

## 1.0.9

- update dependencies

## 1.0.8

- fix panic introduced in 1.0.7

## 1.0.7

- ignore failed delete of non-existent rule

## 1.0.6

- Only consider ip addresses of interfaces that are up in GetOwnIPs

## 1.0.5

- Avoid duplicate ip/tc rules when includes or excludes are not unique

## 1.0.4

- Add utilty to resolve hostnames using dig

## 1.0.3

- Error message for `tc` and rate settings below `8bit/s`

## 1.0.2

- Ignore "Cannot find device" tc error on delete

## 1.0.1

- Add utilities for dealing with tc errors

## 1.0.0

- Initial release

