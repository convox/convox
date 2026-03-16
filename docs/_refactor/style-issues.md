# Phase 1: Style Issues Catalog

Generated: 2026-03-16

---

## 1. Code Block Language Tags

**Severity: High** | **Scope: 107 files, ~450+ occurrences**

Nearly every code example in the docs uses ` ```html ` as the language tag regardless of actual content. This affects syntax highlighting and reader trust.

### Files with highest occurrence counts

| File | Count | Actual content types |
|------|-------|---------------------|
| `reference/cli/rack.md` | 26 | bash, yaml |
| `installation/production-rack/azure.md` | 20 | bash, hcl/terraform |
| `reference/primitives/app/resource/README.md` | 19 | yaml, bash |
| `reference/cli/apps.md` | 18 | bash |
| `deployment/scaling.md` | 17 | yaml, bash |
| `reference/primitives/app/release.md` | 15 | bash, yaml |

### Correct tag mapping needed

- Shell commands (`convox ...`, `curl`, `pip`, etc.) -> `bash`
- YAML config (`convox.yml`, Kubernetes manifests) -> `yaml`
- Terraform/HCL -> `hcl`
- JSON output -> `json`
- Ruby code -> `ruby`
- Python code -> `python`
- JavaScript/Node.js code -> `javascript`
- Dockerfiles -> `dockerfile`
- Plain text output / mixed content -> `text` or remove tag

---

## 2. Frontmatter Inconsistencies

### 2a. `draft: false` present everywhere

**Scope: 161 files**

Every file includes `draft: false` which is the default and adds noise. Should be removed from all files.

### 2b. Slugs not kebab-case

**Scope: ~50+ files**

Many slugs use Title Case, spaces, or special characters instead of kebab-case:

| Pattern | Example Files | Current Slug |
|---------|---------------|-------------|
| Title Case | `deployment/scaling.md` | `Scaling` |
| Spaces | `configuration/app-settings.md` | `App Settings` |
| Spaces | `configuration/environment.md` | `Environment Variables` |
| Special chars | `configuration/monitoring.md` | `Monitoring & Alerting` |
| Mixed case | `example-apps/nodejs.md` | `Node.js` |
| PascalCase | `configuration/app-parameters/aws/build-cpu.md` | `BuildCpu` |
| Mixed | `installation/production-rack/azure.md` | `Microsoft Azure` |
| Mixed | `installation/development-rack/macos-m1.md` | `macOS-M1` |

### 2c. Frontmatter field ordering inconsistent

Most files follow `title, draft, slug, url` but `installation/production-rack/azure.md` has an extra blank line and `configuration/rack-parameters/aws/ssl_ciphers.md` has `draft: false` on line 4 instead of line 3.

---

## 3. Em Dashes and En Dashes

**Scope: 13 files, ~30 occurrences**

### Unicode em dashes (U+2014: "...") found in:

| File | Context |
|------|---------|
| `reference/quick-apps/README.md` | "...apps...leveraging all the scalability" |
| `reference/convox-k8s-mapping.md` | Multiple YAML comment lines using " ... " as separator |
| `configuration/rack-parameters/aws/nvidia_device_plugin_enable.md` | "cannot be fractionally allocated...each container" |
| `configuration/rack-parameters/aws/convox_domain_tls_cert_disable.md` | "functionality of your applications...they will still" |
| `configuration/rack-parameters/azure/nvidia_device_time_slicing_replicas.md` | "software-level feature ... it does not" |

### Unicode en dashes (U+2013: "...") found in:

| File | Context |
|------|---------|
| `reference/quick-apps/README.md` | "Rapid Deployment ... Set up" (bullet list) |
| `reference/quick-apps/drupal.md` | "128MB ... Suitable for small" (size descriptions) |
| `installation/development-rack/macos.md` | `sudo rm ...rf` (broken command!) |
| `installation/development-rack/macos-m1.md` | `sudo rm ...rf` (broken command!) |
| `installation/development-rack/ubuntu.md` | `sudo rm ...rf` (broken command!) |
| `configuration/rack-to-rack.md` | `--r rackNAME` and `--a appNAME` (broken flags!) |

**Critical**: The en dashes in `rm -rf` and CLI flags are actual bugs, not just style issues. The en dash character is not a valid hyphen and these commands will fail if copy-pasted.

---

## 4. Navigation Issues

### 4a. Missing pages (33 files not in nav)

See `audit.md` for full list. Most significant gaps:
- Entire development rack section (4 pages)
- 17 Azure rack parameter pages
- `development/running-locally.md`
- `tutorials/local-development.md`
- `management/password-security.md`
- `configuration/app-settings.md`
- `reference/cli/certs.md`

### 4b. Path mismatches (6)

Nav paths that do not match file `url:` frontmatter values. See `audit.md` for details.

### 4c. Typo

`navigation.json` line 727: `"Postgress"` should be `"PostgreSQL"`.

---

## 5. Heading Hierarchy Issues

### 5a. Missing H1 on some pages

Some pages (particularly rack parameter pages) start content directly after frontmatter without an H1. The H1 comes from the `title:` frontmatter but should also be present in the markdown body for consistency.

### 5b. H1 mismatch with title

Some files have H1 text that does not match the frontmatter `title:`.

### 5c. Heading level skips

Some pages jump from H2 to H4 or use inconsistent nesting. A thorough heading audit should be done in Phase 2.

---

## 6. Trailing Whitespace

### Files with trailing whitespace in frontmatter

| File | Field |
|------|-------|
| `configuration/rack-to-rack.md` | `url:` and `slug:` have trailing spaces |
| `deployment/scaling.md` | Multiple code block lines have trailing spaces |
| Multiple rack parameter files | Minor trailing whitespace |

---

## 7. Stale Content

### 7a. Docker Desktop version pinning

`help/known-issues.md` references Docker Desktop 4.2.0 which is years old. Should be updated or noted as historical.

### 7b. Version 2 / Generation 1 references

| File | Content |
|------|---------|
| `help/changes.md` | "Generation 1 Apps are no longer supported", "Version 2" sections |
| `help/upgrading.md` | v2->v3 upgrade instructions |
| `reference/primitives/app/service.md` | "Only applies for version 2 rack services" |
| `reference/cli/instances.md` | "For v2 rack:" section |

These should be evaluated for relevance. Most v2 users have likely migrated by now.

---

## 8. URL Inconsistencies

### Trailing slashes

`reference/quick-apps/README.md` and `reference/quick-apps/drupal.md` have trailing slashes in their `url:` values. No other files do this.

### Case mismatch

App parameter URLs use PascalCase (`/BuildCpu`) while nav paths use kebab-case (`/build-cpu`). These need to be aligned.

---

## 9. Content Quality Issues

### Very short pages (under 100 words)

| File | Words | Notes |
|------|-------|-------|
| `help/support.md` | 36 | May benefit from expansion |
| `integrations/monitoring/README.md` | 40 | Just a section intro |
| `configuration/rack-parameters/azure/max_on_demand_count.md` | 47 | Minimal content |
| `configuration/rack-parameters/azure/min_on_demand_count.md` | 53 | Minimal content |
| `installation/development-rack/README.md` | 63 | Just links to subpages |
| `configuration/rack-parameters/azure/node_disk.md` | 65 | Minimal content |
| `reference/primitives/app/object.md` | 39 | Stub page |
| `reference/primitives/rack/instance.md` | 66 | Minimal content |
| `configuration/private-registries.md` | 73 | Minimal content |
| `reference/primitives/rack/registry.md` | 76 | Minimal content |

### Typos found

- `tutorials/deploying-an-application.md`: "applicaton" (twice)
- `tutorials/local-development.md`: "applicaton"
- `navigation.json`: "Postgress" should be "PostgreSQL"

---

## 10. Missing Cross-Links and "Next Steps"

Most pages end abruptly without linking to related content. Pages that would most benefit from "Next Steps" sections:

- `getting-started/introduction.md` -> tutorials
- `tutorials/preparing-an-application.md` -> deploying tutorial (has this already)
- `configuration/convox-yml.md` -> specific config pages
- `installation/cli.md` -> dev rack or prod rack install
- `deployment/deploying-changes.md` -> rolling updates, rollbacks
- `configuration/environment.md` -> app settings, convox.yml
- All rack parameter index pages -> individual parameter pages (some have this)

---

## Priority Order for Fixes

1. **Critical**: Fix en dashes in CLI commands (broken commands in 4 files)
2. **High**: Fix ~450 incorrect ````html` code block tags
3. **High**: Add 17 Azure rack parameters to navigation.json
4. **High**: Fix 6 nav path mismatches
5. **Medium**: Remove `draft: false` from all 161 files
6. **Medium**: Standardize slugs to kebab-case
7. **Medium**: Remove em/en dashes throughout
8. **Medium**: Remove duplicate `README.md`
9. **Medium**: Merge macOS dev rack pages
10. **Low**: Add "Next Steps" sections
11. **Low**: Fix trailing whitespace
12. **Low**: Evaluate stale v2 content
