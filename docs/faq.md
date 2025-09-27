# FAQ

## I registered my action provider, but I cannot see the action within the experiment editor?

There are two common issues causing this:

1. The agent couldn't (properly) communicate with the extension. To analyze this further, please inspect the agent log.
2. Your team within Steadybit is not allowed to use the actions. Inspect the team configuration via the Steadybit settings views to
   ensure that the actions are allowed for your team.

## Network Filtering

### Why is my TCP PSH filtering not working as expected?

TCP PSH filtering relies on specific assumptions about network packet structure. Common issues include:

1. **IP Options or IPv6 Extension Headers**: The filtering assumes standard header sizes (20 bytes for IPv4, 40 bytes for IPv6). If your network uses IP options or IPv6 extension headers, the packet offsets will be incorrect.

2. **PSH Flag Not Set**: Some applications or network stacks may not set the PSH flag on data packets. Verify that your test traffic actually has the PSH flag set using `tcpdump`.

3. **Wrong Packet Types**: PSH filtering only affects TCP packets with the PSH flag set and all UDP packets. Control packets (SYN, FIN, ACK-only) typically don't have the PSH flag.

**Troubleshooting steps:**
- Use `tcpdump -i eth0 -n tcp and port 80` to verify PSH flags
- Check `tc filter show dev eth0` to verify the rules are applied
- Test with simple HTTP requests that should generate PSH packets

### Can I use TCP PSH filtering with IPv6?

Yes, TCP PSH filtering supports both IPv4 and IPv6. However, the same packet structure assumptions apply:
- IPv6 assumes standard 40-byte headers with no extension headers
- If IPv6 extension headers are present, the filtering offsets will be incorrect

### What happens to UDP packets when TCP PSH filtering is enabled?

When `TcpPshOnly` is enabled:
- **TCP packets**: Only delayed if they have the PSH flag set
- **UDP packets**: All UDP packets are delayed (UDP has no PSH equivalent)
- **Other protocols**: Not affected by the filter

This behavior ensures that both TCP data packets and UDP packets are consistently delayed, providing predictable network behavior for applications using either protocol.