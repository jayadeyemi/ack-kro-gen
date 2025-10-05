# ack-kro-gen

A production-ready Go CLI that generates **KRO ResourceGraphDefinitions (RGDs)** for AWS ACK controllers. It automates the end-to-end process of pulling upstream ACK Helm charts via the Helm SDK, rendering them fully in-memory, performing placeholder substitution, classifying objects by type, and emitting deterministic graph definitions under `out/ack/`.

## Goal
The project bridges **AWS ACK controllers** (delivered as Helm charts) with **KRO ResourceGraphDefinitions**, enabling:
- Automated generation of consistent KRO RGDs for ~39 AWS services.
- Deterministic and reproducible manifests, with CRDs always ordered before workloads.
- Schema-driven placeholders for account IDs, regions, IRSA providers, and other deployment parameters.
- Offline reproducibility for CI/CD and air‑gapped environments.

This allows DevOps and platform engineers to model, version, and deploy ACK controllers declaratively via KRO without manually maintaining YAML for each service.

## Features
- Go 1.22+
- Pure Helm SDK usage (no shelling out)
- OCI chart support with public ECR integration
- Offline mode with local chart cache (`.cache/charts/`)
- Deterministic output ordering and formatting
- Strict error handling and logging
- Modular design (`config`, `helmfetch`, `render`, `classify`, `placeholders`, `kro`)
- Agent-based validation testing with offline chart caching
- Concurrent processing of multiple services with configurable concurrency

## Architecture

The CLI follows a deterministic pipeline architecture:

```
graphs.yaml → config → helmfetch → .cache/charts/*.tgz → render → classify → placeholders → kro → out/ack/*.yaml
```

**Pipeline Stages:**

1. **Configuration** (`config`): Parse `graphs.yaml` with smart defaults
2. **Chart Fetching** (`helmfetch`): Download/cache OCI charts from AWS ECR
3. **Rendering** (`render`): Helm SDK rendering + Phase 1 placeholders (literals → sentinels)
4. **Classification** (`classify`): Group objects by type with deterministic ordering
5. **Placeholder Transformation** (`placeholders`): Phase 2 (sentinels → schema references)
6. **RGD Generation** (`kro`): Build ResourceGraphDefinitions with schema integration
7. **Output**: Write `<service>-crds.yaml` and `<service>-ctrl.yaml` per service

**Key Design Principles:**
- No network access after chart caching (enables offline/air-gapped usage)
- Deterministic output ordering (CRDs → Core → RBAC → Deployments → Others)
- Two-phase placeholder system preserving YAML structure
- Concurrent multi-service processing with error isolation

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

## graphs.yaml Schema
Each service entry must define the ACK `service` name and the chart `version`. All other fields are optional overrides with smart defaults:

**Required fields:**
- `service`: AWS service name (e.g., `s3`, `ec2`, `rds`)
- `version`: Chart version to pull from `public.ecr.aws/aws-controllers-k8s/<service>-chart`

**Default values (auto-populated if not specified):**
- `releaseName`: `ack-<service>-controller`
- `namespace`: `ack-system`
- `image.repository`: `public.ecr.aws/aws-controllers-k8s/<service>-controller`
- `image.tag`: Falls back to `version` if not specified
- `serviceAccount.create`: `true`
- `enableCARM`: `true`
- `featureGates.ReadOnlyResources`: `true`
- `featureGates.ResourceAdoption`: `true`

**Minimal example:**
```yaml
graphs:
  - service: s3
    version: "1.1.1"

  - service: ec2
    version: "1.7.0"

  - service: rds
    version: "1.6.2"
```

**Full example with overrides:**
```yaml
graphs:
  - service: s3
    version: "1.1.1"
    releaseName: "custom-s3-controller"  # Override default
    namespace: "custom-namespace"        # Override default
    image:
      repository: "custom-repo/s3-controller"
      tag: "v1.1.1"
      pullPolicy: "IfNotPresent"
    serviceAccount:
      create: true
      name: "custom-sa-name"
      annotations:
        eks.amazonaws.com/role-arn: "arn:aws:iam::123456789012:role/s3-controller"
    aws:
      region: "us-west-2"
      endpoint_url: "https://s3.us-west-2.amazonaws.com"
    log:
      level: "debug"
      enable_development_logging: true
    deployment:
      replicas: 2
      containerPort: 8080
      annotations:
        prometheus.io/scrape: "true"
    resources:
      requests:
        memory: "256Mi"
        cpu: "100m"
      limits:
        memory: "512Mi"
        cpu: "200m"
    reconcile:
      defaultResyncPeriod: 600
      defaultMaxConcurrentSyncs: 5
```

See `internal/config/config.go` for the complete `ValuesSpec` structure and all available fields.

## Output Structure
The CLI generates two RGD files per service in the `out/ack/` directory:
- `<service>-crds.yaml`: CustomResourceDefinition graph for the service
- `<service>-ctrl.yaml`: Controller resources graph (ServiceAccount, RBAC, Deployment, etc.)

Example output for S3:
```
out/
└── ack/
    ├── s3-crds.yaml    # CRDs for S3 resources
    ├── s3-ctrl.yaml    # S3 controller deployment
    ├── ec2-crds.yaml   # CRDs for EC2 resources
    └── ec2-ctrl.yaml   # EC2 controller deployment
```

## Adding a Service
1. Add an entry to `graphs.yaml` with `service` and `version` (minimum required)
2. Override defaults only when needed (see `config.ValuesSpec` for available fields)
3. Run the CLI: `./ack-kro-gen --graphs graphs.yaml --out out --charts-cache .cache/charts`
4. Find generated RGDs in `out/ack/<service>-crds.yaml` and `out/ack/<service>-ctrl.yaml`

## Offline mode
- Pre-populate `--charts-cache` with the required ACK charts (either from a prior online run or manual download).
- Use `--offline=true` to render without network calls.

## Determinism
- Objects ordered: CRDs → core resources (SA, ConfigMap, Service, Namespace) → RBAC → Deployments → others.
- Stable resource IDs: `<kind>-<name>` kebab‑cased and deduplicated.
- Canonical YAML encoding ensures reproducible diffs.

## Validation & Testing
Validation is performed via agent-based testing using the `./go.sh` convenience script:

```bash
./go.sh  # Builds, runs generator, and validates outputs
```

The script:
1. Cleans previous outputs
2. Builds the CLI
3. Runs the generator with offline cached charts
4. Validates generated RGDs for:
   - Placeholder substitution correctness
   - Deterministic resource ordering
   - Schema reference generation accuracy
   - Completeness against chart contents
   - Valid KRO ResourceGraphDefinition structure

**Environment variables for `./go.sh`:**
- `BIN`: CLI binary path (default: `./ack-kro-gen`)
- `GRAPHS`: Graph config file (default: `graphs.yaml`)
- `OUT`: Output directory (default: `out`)
- `CACHE`: Chart cache directory (default: `.cache/charts`)
- `CONCURRENCY`: Parallel services (default: `4`)
- `LOG_LEVEL`: Log verbosity (default: `debug`)
- `OFFLINE`: Offline mode (default: `false`)

Example:
```bash
OFFLINE=true CONCURRENCY=8 ./go.sh
```
