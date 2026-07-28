# Flow Stitch Usage

`flow_stitch` is a conversation-oriented output format for flows. It groups records by unordered IP pair, protocol, well-known port, and device, then emits one stitched conversation record only after 5 minutes of inactivity.

## What it does

- Normalizes client and server direction into a single conversation.
- Uses `protocol` and `wellknown_port` instead of carrying both L4 ports through to output.
- Chooses a canonical source and destination for the conversation and sets `sourceinitiated` based on the initiator heuristic.
- Aggregates bytes, packets, TCP flags, and record counts separately in each direction.
- Emits nothing until the conversation has been idle for 5 minutes.

## Output fields

Each emitted JSON record includes these key fields:

- `sourceipaddress`
- `destinationipaddress`
- `sourceinitiated`
- `protocol`
- `wellknown_port`
- `tcp_flags`
- `tcp_flags_rev`
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
- `tcp_flags` and `tcp_flags_rev` are additive within each direction using bitwise OR.

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

## Operational notes

- A conversation is keyed by unordered source and destination IPs, protocol, well-known port, and device name.
- Changes in ephemeral client ports do not split the conversation as long as the well-known service port remains the same.
- Because records are emitted only after 5 minutes of inactivity, a quiet test may appear to produce no output until the TTL expires.