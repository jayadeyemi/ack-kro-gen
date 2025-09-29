# ack-kro-gen

A production-ready Go CLI that generates **KRO ResourceGraphDefinitions (RGDs)** for AWS ACK controllers. It automates the end-to-end process of pulling upstream ACK Helm charts via the Helm SDK, rendering them fully in-memory, performing placeholder substitution, classifying objects by type, and emitting deterministic graph definitions under `out/<service>.rgd.yaml`.

## Goal
The project bridges **AWS ACK controllers** (delivered as Helm charts) with **KRO ResourceGraphDefinitions**, enabling:
- Automated generation of consistent KRO RGDs for ~39 AWS services.
- Deterministic and reproducible manifests, with CRDs always ordered before workloads.
- Schema-driven placeholders for account IDs, regions, IRSA providers, and other deployment parameters.
- Offline reproducibility for CI/CD and air‑gapped environments.

This allows DevOps and platform engineers to model, version, and deploy ACK controllers declaratively via KRO without manually maintaining YAML for each service.

## Features
- Go 1.22
- Pure Helm SDK usage (no shelling out)
- OCI chart support
- Offline mode with a local chart cache
- Deterministic output ordering and formatting
- Strict error handling and logging
- Modular design (`config`, `helmfetch`, `render`, `classify`, `placeholders`, `kro`)
- Unit tests with no network access

## Install
Build the CLI from the `cmd` directory:

```bash
go build -o ack-kro-gen ./cmd/ack-kro-gen
```

## Usage
Generate RGDs online (charts pulled into cache if missing):

- Full run (6 concurrency, info logging):
```bash
./ack-kro-gen --charts-cache .cache/charts --offline=false --graphs graphs.yaml --out out --concurrency 6 --log-level info 
```

Run in offline mode (charts must already exist in the cache):
```bash
./ack-kro-gen --charts-cache .cache/charts --offline=true --graphs graphs.yaml --out out
```

### Notes
- `go build ./...` only checks that all packages compile; it discards binaries. Use `go build ./cmd/ack-kro-gen` or add `-o ack-kro-gen` to produce the CLI executable.
- Install globally with:
  ```bash
  go install ./cmd/ack-kro-gen
  ```
  which drops the binary into `$GOBIN` or `$GOPATH/bin` so you can run `ack-kro-gen` anywhere.

## graphs.yaml schema
Each service entry defines the chart version, release parameters, and overrides. Example:

```yaml
graphs:
  - service: s3
    version: "1.2.27"
    releaseName: "__KRO_NAME__"
    namespace: "__KRO_NS__"
    image:
      repository: "__KRO_IMAGE_REPO__"
      tag: "__KRO_IMAGE_TAG__"
    serviceAccount:
      name: "__KRO_SA_NAME__"
      annotations:
        eks.amazonaws.com/role-arn: "__KRO_IRSA_ARN__"
    controller:
      logLevel: "__KRO_LOG_LEVEL__"
      logDev: "__KRO_LOG_DEV__"
      awsRegion: "__KRO_AWS_REGION__"
    extras:
      values: {}
```

## Adding a service
1. Append a new entry in `graphs.yaml` with `service`, `version`, `releaseName`, and `namespace`. Optional fields allow overriding image, service account, and controller flags.
2. Run the CLI with your cache and output paths.

## Offline mode
- Pre-populate `--charts-cache` with the required ACK charts (either from a prior online run or manual download).
- Use `--offline=true` to render without network calls.

## Determinism
- Objects ordered: CRDs → core resources (SA, ConfigMap, Service, Namespace) → RBAC → Deployments → others.
- Stable resource IDs: `<kind>-<name>` kebab‑cased and deduplicated.
- Canonical YAML encoding ensures reproducible diffs.

## Tests
- Placeholder substitution only modifies scalar strings, preserving structure.
- Classification ensures objects are grouped and ordered consistently.
- End-to-end offline render validated with a local dummy chart in `internal/render/testdata/dummychart`.
