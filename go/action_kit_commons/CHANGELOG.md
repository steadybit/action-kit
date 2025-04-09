# Changelog

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

## 1.1.8

- Remove dependency on lsns

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

