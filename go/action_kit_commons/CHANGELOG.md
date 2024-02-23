# Changelog

## 1.1.5

- avoid collisions for cgroups used by stress-ng attacks
- workaround for failing lsns

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

