# Phase 1 Audit: File Inventory and Proposed Actions

Generated: 2026-03-16

## Summary

- **Total files**: 163 markdown files
- **Total word count**: 95,339
- **Files with `draft: false`**: 161 (all except `reference/convox-k8s-mapping.md` and one or two rack param files that may differ)
- **Files using ````html` incorrectly**: 107 files, ~450+ occurrences
- **Files with em/en dashes**: 13 files, ~30 occurrences
- **Files missing from navigation.json**: 33

---

## File Inventory

### Getting Started

| File | Words | Action | Notes |
|------|-------|--------|-------|
| `README.md` | 1,418 | **Remove** | Exact duplicate of `getting-started/introduction.md` (same url: frontmatter). Redirect to `/getting-started/introduction` |
| `getting-started/introduction.md` | 1,418 | **Keep** | Canonical entry point |

### Tutorials

| File | Words | Action | Notes |
|------|-------|--------|-------|
| `tutorials/preparing-an-application.md` | 1,167 | Keep | Fix code blocks (`html` -> `yaml`/`bash`) |
| `tutorials/deploying-an-application.md` | 889 | Keep | Fix code blocks, typo "applicaton" |
| `tutorials/local-development.md` | 1,069 | Keep | Fix code blocks, typo "applicaton". Not in nav |

### Example Apps

| File | Words | Action | Notes |
|------|-------|--------|-------|
| `example-apps/README.md` | 821 | Keep | Fix code blocks |
| `example-apps/django.md` | 590 | Keep | Fix code blocks |
| `example-apps/nodejs.md` | 368 | Keep | Fix code blocks |
| `example-apps/rails.md` | 1,032 | Keep | Fix code blocks (`html` -> `yaml`/`bash`/`ruby`) |

### Cloud

| File | Words | Action | Notes |
|------|-------|--------|-------|
| `cloud/README.md` | 801 | Keep | Well-written, minor style fixes |
| `cloud/getting-started.md` | 992 | Keep | Well-written |
| `cloud/cli-reference.md` | 2,540 | Keep | Well-written |
| `cloud/comparison.md` | 1,335 | Keep | Well-written |
| `cloud/migration-guide.md` | 1,717 | Keep | Well-written |
| `cloud/machines/README.md` | 1,212 | Keep | Well-written |
| `cloud/machines/sizing-and-pricing.md` | 880 | Keep | Well-written |
| `cloud/machines/limitations.md` | 1,672 | Keep | Well-written |
| `cloud/databases/README.md` | 641 | Keep | Well-written |
| `cloud/databases/postgres.md` | 554 | Keep | Well-written |
| `cloud/databases/mysql.md` | 531 | Keep | Well-written |
| `cloud/databases/mariadb.md` | 573 | Keep | Well-written |
| `cloud/databases/sizing-and-pricing.md` | 814 | Keep | Well-written |

### Configuration (Core)

| File | Words | Action | Notes |
|------|-------|--------|-------|
| `configuration/convox-yml.md` | 331 | Keep | Fix code blocks |
| `configuration/dockerfile.md` | 291 | Keep | Fix code blocks |
| `configuration/environment.md` | 587 | Keep | Fix code blocks |
| `configuration/health-checks.md` | 1,524 | Keep | Fix code blocks |
| `configuration/load-balancers.md` | 683 | Keep | Fix code blocks |
| `configuration/logging.md` | 104 | Keep | Very short, may need expansion |
| `configuration/monitoring.md` | 1,019 | Keep | Slug has `&` character, needs fix |
| `configuration/private-registries.md` | 73 | Keep | Very short (73 words) |
| `configuration/service-discovery.md` | 226 | Keep | Fix code blocks |
| `configuration/rack-to-rack.md` | 279 | Keep | Has en dashes in commands (bug), trailing whitespace in url |
| `configuration/volumes.md` | 710 | Keep | Fix code blocks |
| `configuration/agents.md` | 204 | Keep | Fix code blocks |
| `configuration/app-settings.md` | 223 | Keep | Not in nav, slug has space ("App Settings") |
| `configuration/workload-placement.md` | 1,904 | Keep | Fix code blocks |

### Configuration: Rack Parameters

| File | Words | Action | Notes |
|------|-------|--------|-------|
| `configuration/rack-parameters/README.md` | 514 | Keep | Fix code blocks |
| `configuration/rack-parameters/aws/README.md` | 796 | Keep | Fix code blocks |
| `configuration/rack-parameters/aws/*.md` (42 files) | ~200-900 each | **Keep all** | Standardize section order per style guide. Fix ````html` tags |
| `configuration/rack-parameters/azure/README.md` | 309 | Keep | Fix code blocks |
| `configuration/rack-parameters/azure/*.md` (21 files) | ~47-816 each | **Keep all** | 17 files missing from nav. Standardize section order |
| `configuration/rack-parameters/gcp/README.md` | 110 | Keep | |
| `configuration/rack-parameters/gcp/*.md` (5 files) | ~144-169 each | **Keep all** | |
| `configuration/rack-parameters/do/README.md` | 113 | Keep | |
| `configuration/rack-parameters/do/*.md` (5 files) | ~131-170 each | **Keep all** | |

### Configuration: App Parameters

| File | Words | Action | Notes |
|------|-------|--------|-------|
| `configuration/app-parameters/README.md` | 140 | Keep | |
| `configuration/app-parameters/aws/README.md` | 226 | Keep | |
| `configuration/app-parameters/aws/build-cpu.md` | 433 | Keep | URL mismatch: file is `build-cpu.md` but url is `/BuildCpu`. Nav path `/build-cpu` does not match url `/BuildCpu` |
| `configuration/app-parameters/aws/build-labels.md` | 461 | Keep | Same URL mismatch issue |
| `configuration/app-parameters/aws/build-mem.md` | 469 | Keep | Same URL mismatch issue |

### Deployment

| File | Words | Action | Notes |
|------|-------|--------|-------|
| `deployment/deploying-changes.md` | 281 | Keep | Fix code blocks |
| `deployment/rolling-updates.md` | 227 | Keep | |
| `deployment/rollbacks.md` | 222 | Keep | Fix code blocks |
| `deployment/scaling.md` | 1,261 | Keep | Fix code blocks (17 occurrences!) |
| `deployment/ssl.md` | 675 | Keep | Fix code blocks |
| `deployment/custom-domains.md` | 146 | Keep | Fix code blocks |
| `deployment/ci-cd-workflows.md` | 1,151 | Keep | Nav path mismatch: nav says `/deployment/ci-cd-workflows`, file url says `/deployment/workflows` |

### Development

| File | Words | Action | Notes |
|------|-------|--------|-------|
| `development/running-locally.md` | 433 | Keep | Fix code blocks. Not in nav |

### Installation

| File | Words | Action | Notes |
|------|-------|--------|-------|
| `installation/cli.md` | 106 | Keep | Fix code blocks |
| `installation/development-rack/README.md` | 63 | Keep | Not in nav |
| `installation/development-rack/macos.md` | 311 | **Evaluate merge** | Nearly identical to macos-m1.md. Only differs by chip arch label. Not in nav |
| `installation/development-rack/macos-m1.md` | 311 | **Evaluate merge** | Nearly identical to macos.md. Not in nav |
| `installation/development-rack/ubuntu.md` | 305 | Keep | Not in nav. Has en dash in command |
| `installation/production-rack/README.md` | 106 | Keep | |
| `installation/production-rack/aws.md` | 1,593 | Keep | Fix code blocks |
| `installation/production-rack/gcp.md` | 286 | Keep | Fix code blocks |
| `installation/production-rack/azure.md` | 1,377 | Keep | Fix code blocks (20 occurrences!) |
| `installation/production-rack/do.md` | 196 | Keep | Fix code blocks |

### Management

| File | Words | Action | Notes |
|------|-------|--------|-------|
| `management/run.md` | 236 | Keep | Fix code blocks |
| `management/cli-rack-management.md` | 746 | Keep | Fix code blocks |
| `management/console-rack-management.md` | 922 | Keep | Fix code blocks |
| `management/deploy-keys.md` | 345 | Keep | |
| `management/rbac.md` | 1,308 | Keep | |
| `management/direct-k8s-access.md` | 421 | Keep | |
| `management/password-security.md` | 236 | Keep | Not in nav. New file (untracked) |

### Reference: CLI

| File | Words | Action | Notes |
|------|-------|--------|-------|
| `reference/cli/README.md` | 373 | Keep | |
| `reference/cli/*.md` (27 subcommand files) | 33-935 each | **Keep all** | Fix code blocks throughout. `certs.md` missing from nav |
| Notable: `reference/cli/rack.md` | 935 | Keep | Highest code block fix count (26) |

### Reference: Primitives

| File | Words | Action | Notes |
|------|-------|--------|-------|
| `reference/primitives/README.md` | 287 | Keep | |
| `reference/primitives/app/README.md` | 250 | Keep | |
| `reference/primitives/app/service.md` | 2,744 | Keep | Largest primitive page |
| `reference/primitives/app/resource/README.md` | 1,353 | Keep | Fix code blocks |
| `reference/primitives/app/resource/*.md` (6 files) | 67-847 each | **Keep all** | Fix code blocks |
| `reference/primitives/rack/README.md` | 153 | Keep | |
| `reference/primitives/rack/*.md` (2 files) | 66-76 each | **Keep all** | |

### Reference: Other

| File | Words | Action | Notes |
|------|-------|--------|-------|
| `reference/convox-k8s-mapping.md` | 3,583 | Keep | Has many em dashes (in YAML comments). Largest reference page |
| `reference/quick-apps/README.md` | 190 | Keep | Has em dashes. URL has trailing slash |
| `reference/quick-apps/drupal.md` | 2,367 | Keep | Has en dashes. URL has trailing slash |

### Integrations

| File | Words | Action | Notes |
|------|-------|--------|-------|
| `integrations/monitoring/README.md` | 40 | Keep | Very short |
| `integrations/monitoring/datadog.md` | 448 | Keep | |
| `integrations/headless-cms/README.md` | 141 | Keep | |
| `integrations/headless-cms/contentful.md` | 611 | Keep | |

### Help

| File | Words | Action | Notes |
|------|-------|--------|-------|
| `help/troubleshooting.md` | 926 | Keep | |
| `help/known-issues.md` | 175 | Keep | Stale Docker Desktop 4.2.0 reference |
| `help/changes.md` | 252 | **Evaluate** | v2->v3 migration content. References "Generation 1" |
| `help/upgrading.md` | 236 | **Evaluate** | v2->v3 migration content. Could link to `help/changes.md` |
| `help/support.md` | 36 | Keep | Very short |

---

## Navigation Gaps

### Pages in docs but NOT in navigation.json (33 files)

**Development rack pages** (entire section missing from nav):
- `installation/development-rack/README.md`
- `installation/development-rack/macos.md`
- `installation/development-rack/macos-m1.md`
- `installation/development-rack/ubuntu.md`

**Development section**:
- `development/running-locally.md`

**Tutorials**:
- `tutorials/local-development.md`

**Management**:
- `management/password-security.md` (new, untracked file)

**Configuration**:
- `configuration/app-settings.md`

**Azure rack parameters** (17 files missing from nav):
- `additional_build_groups_config`, `additional_node_groups_config`, `high_availability`, `idle_timeout`, `internal_router`, `max_on_demand_count`, `min_on_demand_count`, `nginx_additional_config`, `nginx_image`, `node_disk`, `nvidia_device_plugin_enable`, `nvidia_device_time_slicing_replicas`, `pdb_default_min_available_percentage`, `proxy_protocol`, `ssl_ciphers`, `ssl_protocols`, `tags`

**AWS rack parameters** (1 file missing):
- `kubelet_registry_pull_params.md`

**CLI reference**:
- `reference/cli/certs.md`

### Nav paths that point to non-existent URLs (6 mismatches)

| Nav Path | Issue |
|----------|-------|
| `/configuration/app-parameters/aws/build-cpu` | File url is `/configuration/app-parameters/aws/BuildCpu` |
| `/configuration/app-parameters/aws/build-labels` | File url is `/configuration/app-parameters/aws/BuildLabels` |
| `/configuration/app-parameters/aws/build-mem` | File url is `/configuration/app-parameters/aws/BuildMem` |
| `/deployment/ci-cd-workflows` | File url is `/deployment/workflows` |
| `/reference/quick-apps` | File url is `/reference/quick-apps/` (trailing slash) |
| `/reference/quick-apps/drupal` | File url is `/reference/quick-apps/drupal/` (trailing slash) |

### Typo in navigation.json

- Line 727: `"Postgress"` should be `"PostgreSQL"`

---

## Merge/Remove Decisions

| Action | Files | Rationale |
|--------|-------|-----------|
| **Remove** | `README.md` | Exact byte-for-byte duplicate of `getting-started/introduction.md` |
| **Merge** | `installation/development-rack/macos.md` + `macos-m1.md` | Nearly identical (only H1 and slug differ). Combine into single page with sections for Intel vs ARM |
| **Keep separate** | `help/changes.md` + `help/upgrading.md` | Different focus (changelog vs upgrade steps). Add cross-links between them |
| **Keep separate** | All rack parameter pages | Individually indexed and linked |
| **Keep separate** | All CLI reference pages | Standard reference structure |

---

## Key Statistics

- **Code block fixes needed**: ~450+ ````html` tags across 107 files
- **Frontmatter fixes needed**: 161 files have `draft: false` to remove; ~50+ files have non-kebab-case slugs
- **Em/en dash removals**: ~30 occurrences across 13 files
- **Navigation additions needed**: 33 files need nav entries (primarily Azure rack params and dev rack pages)
- **Navigation fixes needed**: 6 path mismatches, 1 typo
