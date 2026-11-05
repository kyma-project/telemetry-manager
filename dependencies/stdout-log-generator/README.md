# Stdout Log Generator

A small tool that continuously writes log lines to stdout. Used in the e2e tests of telemetry-manager to produce configurable log traffic inside Kubernetes workloads.

The tool exposes a Prometheus metrics endpoint on port `2112` (`/metrics`) with two metrics:

- `logs_generated_total` — total log lines written since start
- `logs_generated_rate` — actual throughput in logs/second since start

## Usage

```sh
stdout-log-generator [flags]
```

## Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--format` | | `json` | Output format: `json` or `plaintext` |
| `--bytes` | `-b` | `2048` | Size of each log line in bytes |
| `--rate` | `-r` | `1` | Target log lines per second. `0` means unlimited |
| `--fields` | `-f` | | Additional key=value fields to include in every JSON log record (can be repeated or comma-separated). Ignored when `--format plaintext` is used |
| `--text` | `-t` | | Fixed text to write for every plaintext log line. Ignored if `--bytes` is set. Only relevant when `--format plaintext` is used |

## Output formats

### JSON (default)

Each line is a JSON object with a `padding` field filled with random characters to reach the requested byte size, plus any custom fields added via `--fields`:

```json
{"padding":"aBcDeFgH...","environment":"prod","app":"myservice"}
```

The `padding` field is sized so that the total serialized JSON line matches `--bytes` exactly (including the trailing newline added by the JSON encoder).

If the combined size of the custom fields already exceeds `--bytes`, the tool exits with an error.

### Plaintext

Each line is either the fixed string given by `--text`, or a random string of `--bytes` length when `--text` is not set.

## Rate limiting

The tool uses a token-bucket rate limiter (`golang.org/x/time/rate`). On Linux, the minimum achievable sleep granularity is approximately 2 ms. To compensate, the burst size is set to `2 × (rate / 1000)` so that the generator can emit multiple logs back-to-back before sleeping, keeping the long-run average close to the requested rate even at high throughputs.

Setting `--rate 0` disables throttling entirely (write as fast as possible).

## Examples

Generate 100 JSON logs per second, each 1 KiB in size:

```sh
stdout-log-generator --rate 100 --bytes 1024
```

Generate JSON logs with custom fields at the default rate:

```sh
stdout-log-generator --fields app=myservice,env=prod
```

Generate plaintext logs with a fixed message at 50 logs/second:

```sh
stdout-log-generator --format plaintext --text "hello world" --rate 50
```

Generate random plaintext logs of 512 bytes each, unlimited rate:

```sh
stdout-log-generator --format plaintext --bytes 512 --rate 0
```

## Build locally

```sh
docker build -t stdout-log-generator:local .
```

Run the local image:

```sh
docker run --rm stdout-log-generator:local --rate 10 --fields app=test
```
