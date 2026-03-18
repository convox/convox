# Documentation Refactor Changelog

All changes completed: 2026-03-16

---

## Phase 1: Audit and Plan

**No files modified.** Produced three planning documents:

- `_refactor/audit.md`: Full inventory of 163 markdown files with word counts and proposed actions
- `_refactor/redirects.csv`: Initial redirect candidates
- `_refactor/style-issues.md`: Catalog of 10 categories of formatting inconsistencies

---

## Phase 2: Structural Fixes

### Code Block Language Tags

Fixed ~450 incorrect ` ```html ` language tags across 107 files. Replaced with the correct tag based on content:

- Shell commands: `bash`
- YAML/convox.yml config: `yaml`
- JSON output: `json`
- Plain text/output: `text`
- Dockerfiles: `dockerfile`

**Files affected:** All files that contained code blocks (107 files)

### Frontmatter Standardization

- Removed `draft: false` from all 161 files
- Converted all slugs to kebab-case (from Title Case, spaces, PascalCase, etc.)
- Fixed trailing whitespace in `url:` and `slug:` fields
- Fixed broken frontmatter in `installation/production-rack/azure.md` (corrupted `---` delimiter)
- Fixed `ci/cd-workflows` slug to `ci-cd-workflows`
- Fixed `convox.yml` slug to `convox-yml`
- Removed trailing slashes from `reference/quick-apps/README.md` and `reference/quick-apps/drupal.md` URLs

### Duplicate Removal

- **Deleted** `docs/README.md` (byte-identical duplicate of `getting-started/introduction.md`)

### Critical Bug Fixes

Fixed en-dash characters (U+2013) in CLI commands that would cause copy-paste failures:

| File | Broken Command | Fixed |
|------|---------------|-------|
| `installation/development-rack/macos.md` | `sudo rm -rf` (en-dash) | `sudo rm -rf` (hyphen) |
| `installation/development-rack/macos-m1.md` | `sudo rm -rf` (en-dash) | `sudo rm -rf` (hyphen) |
| `installation/development-rack/ubuntu.md` | `sudo rm -rf` (en-dash) | `sudo rm -rf` (hyphen) |
| `configuration/rack-to-rack.md` | `-r rackNAME`, `-a appNAME` (en-dashes) | `-r rackNAME`, `-a appNAME` (hyphens) |

### Em/En Dash Removal

Removed ~30 em/en dash occurrences across 13 files, replacing with commas, periods, colons, or sentence restructuring:

- `reference/quick-apps/README.md`: em-dashes in bullet list, em-dash in body text
- `reference/quick-apps/drupal.md`: en-dashes in size descriptions
- `reference/convox-k8s-mapping.md`: em-dashes in YAML comments (replaced with ` - `)
- `configuration/rack-parameters/aws/nvidia_device_plugin_enable.md`: em-dash mid-sentence
- `configuration/rack-parameters/aws/convox_domain_tls_cert_disable.md`: em-dash mid-sentence
- `configuration/rack-parameters/azure/nvidia_device_time_slicing_replicas.md`: em-dash mid-sentence

### Navigation Updates

Added to `navigation.json`:

- **17 Azure rack parameter pages** (additional_build_groups_config through tags)
- **1 AWS rack parameter page** (kubelet_registry_pull_params)
- **1 CLI reference page** (certs)
- **Development Rack section** with macOS, macOS-M1, Ubuntu sub-pages
- **Development section** with Running Locally
- **tutorials/local-development** in Tutorials
- **management/password-security** in Management
- **configuration/app-settings** in Configurations

Fixed in `navigation.json`:

- CI/CD Workflows path: `/deployment/ci-cd-workflows` changed to `/deployment/workflows` (matching file URL)
- App parameter paths: changed to PascalCase URLs matching file URLs (`/BuildCpu`, `/BuildLabels`, `/BuildMem`)
- "Postgress" typo corrected to "PostgreSQL"
- "Quick-Apps" title changed to "Quick Apps"
- Formatting fixed (minor indentation and whitespace)

---

## Phase 3: Deduplication and Merges

### macOS Dev Rack Pages Merged

- **Merged** `installation/development-rack/macos.md` + `installation/development-rack/macos-m1.md`
- Content was identical; unified page notes it applies to both Intel and Apple Silicon
- Deleted `macos-m1.md`
- Updated `installation/development-rack/README.md` index to single macOS link
- Removed macOS-M1 entry from `navigation.json`
- **Redirect**: `/installation/development-rack/macos-m1` -> `/installation/development-rack/macos`

### Cross-Links Added

- Added "See Also" section to `help/changes.md` linking to `help/upgrading.md`
- Added "See Also" section to `help/upgrading.md` linking to `help/changes.md`

### Broken Internal Links Fixed

| File | Old Link | Corrected To |
|------|----------|-------------|
| `configuration/app-parameters/aws/README.md` | `/configuration/rack-parameters/aws/additional_build_groups` | `...additional_build_groups_config` |
| `cloud/migration-guide.md` | `/cloud/monitoring` | `/configuration/monitoring` |
| `installation/development-rack/ubuntu.md` | `/reference/primitives/getting-started/introduction/` | `/installation/cli` |
| `example-apps/README.md` | `/deployment` | `/deployment/deploying-changes` |
| `integrations/headless-cms/contentful.md` | `/console/workflows#deployment-workflows` | `/deployment/workflows` |

### Redirects Finalized

Produced `_refactor/redirects-final.csv` with 8 redirect entries covering all URL changes.

---

## Phase 4: Content Standardization and Voice

### Filler Word Removal

Removed all instances of filler words across ~25 files:

| Word/Phrase | Occurrences Removed | Replacement |
|------------|--------------------:|-------------|
| "simply" | 10 | Removed (e.g., "sign up" instead of "simply sign up") |
| "easily" | 6 | Removed (e.g., "scale" instead of "easily scale") |
| "just" (as filler) | 5 | Removed (kept when meaning "only") |
| "Note that" | 5 | Reworded as direct statement |
| "Please note" | 4 | Reworded as direct statement |
| "It's important to note that" | 1 | Reworded as direct statement |

### Cross-Links Added

Added "See Also" sections with 2-3 contextually relevant links to 22 core pages:

- Getting Started: introduction
- Installation: cli, dev-rack (macOS, Ubuntu), production-rack index
- Configuration: convox-yml, environment, health-checks, load-balancers, volumes, logging, monitoring
- Deployment: deploying-changes, rolling-updates, rollbacks, scaling, custom-domains, ssl
- Development: running-locally
- Management: run, cli-rack-management, console-rack-management

**Total pages with cross-links after Phase 4: 46** (including pre-existing "Next Steps" in tutorials, "Related Parameters" in rack param pages, and "See Also" in help pages)

### Azure Rack Parameter Standardization

Expanded 3 minimal Azure rack parameter pages to match the standard section template:

- `configuration/rack-parameters/azure/max_on_demand_count.md`
- `configuration/rack-parameters/azure/min_on_demand_count.md`
- `configuration/rack-parameters/azure/node_disk.md`

Added: Description (expanded), Use Cases, and renamed "Example" to "Setting Parameters" with standard CLI format.

### Code Block Fixes

Added language tags to 9 code blocks that were missing them:

| File | Tag | Content |
|------|-----|---------|
| `reference/convox-k8s-mapping.md` | `text` | ASCII K8s resource diagram |
| `reference/primitives/app/build.md` | `text` | BUILD_AUTH env var |
| `management/console-rack-management.md` | `bash` | kubectl command |
| `cloud/getting-started.md` | `text` | Database env vars |
| `cloud/migration-guide.md` | `text` | Heroku Procfile |
| `cloud/databases/README.md` | `text` | Database env vars |
| `cloud/databases/mariadb.md` | `text` | MariaDB env vars |
| `cloud/databases/mysql.md` | `text` | MySQL env vars |
| `cloud/databases/postgres.md` | `text` | Postgres env vars |

### Heading and Formatting Fixes

- Fixed unclosed code block in `configuration/monitoring.md` (was swallowing content)
- Renamed title "Monitoring & Alerting" to "Monitoring and Alerting" (title, H1, and body)
- Cleaned trailing whitespace across 42 files
- Collapsed double/triple blank lines to single blank lines across all files
- **0 multi-H1 pages remaining**

---

## Phase 5: Navigation Rebuild and Final QA

### Navigation Verification

- Validated `navigation.json` is valid JSON
- 12 sections, 213 total entries (212 unique + 1 intentional duplicate for `/example-apps`)
- All nav paths have corresponding files
- All file URLs appear in navigation
- No orphaned files or nav entries

### Final QA Results

| Check | Result |
|-------|--------|
| Em/en dashes (U+2013, U+2014) | 0 |
| ` ```html ` code blocks | 0 |
| `draft: false` in frontmatter | 0 |
| Filler words (simply/easily) | 0 |
| Broken internal links | 0 |
| Slugs with spaces/special chars | 0 |
| Multi-H1 pages | 0 |
| Code blocks without language tag | 0 |
| Pages with cross-links | 46/212 |

### Navigation Structure Summary

| Section | Visibility | Pages |
|---------|-----------|------:|
| Getting Started | visible | 1 |
| Tutorials | visible | 4 |
| Installation | visible | 9 |
| Configurations | visible | 105 |
| Deployment | visible | 7 |
| Development | visible | 1 |
| Management | visible | 7 |
| Cloud | visible | 13 |
| Reference | visible | 53 |
| Integrations | visible | 4 |
| Help | visible | 5 |
| _hidden | hidden | 4 |
| **Total** | | **213** |

---

## Files Deleted

| File | Reason | Redirect |
|------|--------|----------|
| `docs/README.md` | Exact duplicate of `getting-started/introduction.md` | URL preserved (same `url:` frontmatter) |
| `installation/development-rack/macos-m1.md` | Merged into `macos.md` | `/installation/development-rack/macos` |

## Files Created

| File | Purpose |
|------|---------|
| `_refactor/audit.md` | Phase 1 file inventory and action plan |
| `_refactor/redirects.csv` | Initial redirect candidates |
| `_refactor/style-issues.md` | Formatting inconsistency catalog |
| `_refactor/redirects-final.csv` | Final redirect list (8 entries) |
| `_refactor/changelog.md` | This file |
| `_refactor/fix_phase2.py` | Automated fix script for code blocks and frontmatter |
