# Changelog

## 1.2.5

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

