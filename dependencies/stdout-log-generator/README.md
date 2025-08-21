# Stdout Log Generator

Small tool for generating logs to stdout, used in the e2e tests of the telemetry-manager.

Available command line args:

- format
- bytes
- rate
- fields
- text

## Build locally

To build the image locally run:

```sh
docker build -t stdout-log-generator:local .
```
