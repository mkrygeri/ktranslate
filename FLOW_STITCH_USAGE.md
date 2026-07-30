# Flow Stitch Usage

`flow_stitch` is a conversation-oriented output format for flows. It groups records by unordered IP pair, protocol, well-known port, and device, then emits one stitched conversation record only after 5 minutes of inactivity.

## What it does

- Normalizes client and server direction into a single conversation.
- Uses `protocol` and `wellknown_port` instead of carrying both L4 ports through to output.
- Chooses a canonical source and destination for the conversation and sets `sourceinitiated` based on the initiator heuristic.
- Aggregates bytes, packets, and record counts separately in each direction, and TCP flags as a single combined set.
- Emits nothing until the conversation has been idle for 5 minutes.

## Output fields

Each emitted JSON record includes these key fields:

- `sourceipaddress`
- `destinationipaddress`
- `sourceinitiated`
- `protocol`
- `wellknown_port`
- `tcp_flags`
- `octetdeltacount`
- `octetdeltacount_rev`
- `packetdeltacount`
- `packetdeltacount_rev`
- `cnt`
- `cnt_rev`
- `starttimestamp`
- `endtimestamp`
- `maxduration`
- `devicename`
- `sourceinterface`
- `destinationinterface`

## Important behavior

- The inactivity timeout is fixed at 5 minutes in the current implementation.
- Output is newline-delimited JSON.
- Compression for `flow_stitch` supports only `none` and `gzip`.
- If a flow arrives without a valid protocol value upstream, protocol `0` may be rendered as `HOPOPT`.
- `cnt` is the number of aggregated forward-direction records.
- `cnt_rev` is the number of aggregated reverse-direction records.
- `tcp_flags` is a single set collapsing the TCP flags seen in both directions, combined with bitwise OR.

## Recommended command lines

### HTTP input to stdout

Use this when you are posting flow JSON to ktranslate over HTTP and want the stitched conversation output written to stdout:

```bash
sudo bin/ktranslate \
  -format flow_stitch \
  -compression none \
  -sinks stdout \
  -http.source \
  -stitch.enable=true \
  -stitch.buffer.len 10000 \
  -asn /etc/ktranslate/GeoLite2-ASN.mmdb \
  -geo /etc/ktranslate/GeoLite2-Country.mmdb \
  -listen 0.0.0.0:8085 \
  -log_level info
```

### NetFlow or IPFIX input to stdout

Use this when ktranslate is directly ingesting flow telemetry instead of receiving HTTP JSON:

```bash
sudo bin/ktranslate \
  -format flow_stitch \
  -compression none \
  -sinks stdout \
  -nf.source ipfix \
  -listen 0.0.0.0:9995 \
  -stitch.enable=true \
  -stitch.buffer.len 10000 \
  -asn /etc/ktranslate/GeoLite2-ASN.mmdb \
  -geo /etc/ktranslate/GeoLite2-Country.mmdb \
  -log_level info
```

## When to use `-stitch.enable`

`flow_stitch` itself is the formatter that performs the 5-minute conversation aggregation. The separate `-stitch.enable=true` option enables the earlier flow stitcher in the pipeline, which tries to match ingress and egress flow halves before formatting.

In practice:

- Keep `-stitch.enable=true` when you want the existing pipeline stitching behavior as well.
- You can still use `-format flow_stitch` without `-stitch.enable=true` if your upstream data already contains the flows you want to aggregate.

## HTTP input expectations

When using `-http.source`, the posted JSON must carry a numeric protocol value. For example:

```json
[
  {
    "ts": "2026-07-14T12:00:00Z",
    "vendor": "demo",
    "src_ip": "10.0.0.10",
    "dst_ip": "10.0.0.20",
    "protocol": "6",
    "src_port": "53124",
    "dst_port": "443",
    "src_packets": "12",
    "dst_packets": "9",
    "src_bytes": "1200",
    "dst_bytes": "980"
  }
]
```

If `protocol` is missing or defaults to `0`, output records may show `HOPOPT` because that is the name mapped to protocol number `0`.

## Device name resolution

The `devicename` field is not carried in the incoming flow data. It is filled in by a lookup that matches the sender's IP against a device's sending IPs:

- **NetFlow / IPFIX**: the exporter's sampler (agent) IP is looked up. If there is no match, `devicename` falls back to that sampler IP.
- **HTTP / firehose**: the remote IP of the client posting the JSON is looked up. If there is no match, `devicename` is left blank (there is no IP fallback on this path).

You can provide the device list two ways.

### Option 1: sideload a local device file (no Kentik account needed)

Use `-api_device_file` to load devices from a JSON file without contacting the Kentik API:

```bash
sudo bin/ktranslate \
  -format flow_stitch \
  -compression none \
  -sinks stdout \
  -nf.source ipfix \
  -listen 0.0.0.0:9995 \
  -api_device_file /etc/ktranslate/devices.json \
  -stitch.enable=true \
  -stitch.buffer.len 10000 \
  -log_level info
```

Each device entry must list the sender's IP under `sending_ips`, and the `device_name` is what gets emitted:

```json
{
  "1": {
    "id": "1",
    "company_id": "0",
    "device_name": "router1",
    "device_type": "router",
    "device_status": "V",
    "sending_ips": ["192.0.2.10"],
    "device_sample_rate": "1"
  }
}
```

### Option 2: pull the device list from the Kentik API

Supply credentials and ktranslate pulls the full device list at startup and refreshes it roughly hourly, keyed by each device's sending IPs:

```bash
sudo KENTIK_API_TOKEN=your_token bin/ktranslate \
  -format flow_stitch \
  -compression none \
  -sinks stdout \
  -nf.source ipfix \
  -listen 0.0.0.0:9995 \
  -kentik_email you@example.com \
  -stitch.enable=true \
  -stitch.buffer.len 10000 \
  -log_level info
```

The email is passed with `-kentik_email` and the API token is read from the `KENTIK_API_TOKEN` environment variable (there is no token flag). Note that `sudo` resets the environment, so set the variable *after* `sudo` (as above) or use `sudo -E` — otherwise the token will not reach ktranslate. Each exporter's IP must be registered as a sending IP on a device in your Kentik account for the lookup to match.

### Behind a proxy or load balancer (firehose)

When flow JSON is posted through a proxy (for example HAProxy or a Kubernetes Service), the remote IP seen by ktranslate is the proxy's, not the original sender's. Either add the proxy IP to a device's `sending_ips`, or override the IP used for the lookup:

```bash
  -http.remote_ip 192.0.2.10
```

Tip: run `sudo tcpdump -i <iface> port <listen-port>` to confirm the exact source IP reaching ktranslate, then make sure that IP is in `sending_ips`.

## Operational notes


- A conversation is keyed by unordered source and destination IPs, protocol, well-known port, and device name.
- Changes in ephemeral client ports do not split the conversation as long as the well-known service port remains the same.
- Because records are emitted only after 5 minutes of inactivity, a quiet test may appear to produce no output until the TTL expires.