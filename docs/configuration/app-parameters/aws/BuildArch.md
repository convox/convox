---
title: "BuildArch"
description: "The BuildArch app parameter pins an App's built image to a single CPU architecture, amd64 or arm64, and directs its build pod to a matching build node when dedicated build nodes are enabled."
slug: buildarch
url: /configuration/app-parameters/aws/BuildArch
---

# BuildArch

## What BuildArch Controls
The `BuildArch` app parameter pins an App's built image to a single CPU architecture. The Rack passes the value to the builder as the target platform, so the produced image is built for that architecture regardless of which node the build pod ran on.

`BuildArch` has a second effect that depends on Rack configuration. When the Rack has dedicated build nodes enabled ([`build_node_enabled=true`](/configuration/rack-parameters/aws/build_node_enabled)), the build pod also receives a `kubernetes.io/arch` node affinity that restricts it to a build node of that architecture, so the Build runs natively. With the default `build_node_enabled=false` there is no placement constraint at all: the build pod schedules on any available node, and if that node is a different architecture the image is produced through emulation.

| Effect | Applies when |
|:-------|:-------------|
| The image is built for `linux/<arch>` | Every provider, on any Build the Rack runs. Not applied to `--development` Builds, or to `--external` Builds, which run on your own machine |
| The build pod is restricted to a build node of that architecture | Only when the Rack has `build_node_enabled=true` |

`BuildArch` takes effect on Racks running 3.25.3 or later. On earlier Racks the CLI accepts the value and the Rack discards it: the parameter is not stored, `convox apps params` does not list it, and Builds are unaffected.

## The Two Configurations That Use BuildArch

`BuildArch` serves two different Rack configurations. Which one you are in determines which of the effects above matters.

**Mixed-architecture node groups.** This is what `BuildArch` was introduced for. The Rack runs managed node groups of both architectures, added with [`additional_node_groups_config`](/configuration/rack-parameters/aws/additional_node_groups_config) and [`additional_build_groups_config`](/configuration/rack-parameters/aws/additional_build_groups_config), and each App is pinned to the architecture of the nodes it runs on. Here `BuildArch` does the work it is named for: with `build_node_enabled=true` it sends the App's build pod to a build node of the matching architecture so the Build is native rather than emulated. Pair it with `nodeSelectorLabels` on the Services so the resulting Processes land on workers of the same architecture. The walkthrough below is this configuration.

**A Karpenter Rack building multi-architecture images.** When the Rack's workload architectures span both `amd64` and `arm64`, whether through [`karpenter_arch`](/configuration/rack-parameters/aws/karpenter_arch) or through an [`additional_karpenter_nodepools_config`](/configuration/rack-parameters/aws/additional_karpenter_nodepools_config) entry declaring a different `arch`, every App on the Rack builds a multi-architecture image index by default and no per-App pinning is needed. Here `BuildArch` does the opposite job: it narrows one App back to a single platform, which is the remedy when that App's base image is not published for both architectures. Node placement is not the point in this configuration, and Services need no `nodeSelectorLabels` because a multi-architecture image runs on either kind of node.

The two are not mutually exclusive, and the parameter behaves the same way in both. Only the reason for setting it differs.

## Default Value
By default, `BuildArch` is not set. The App inherits the Rack's build architecture. On a single-architecture Rack that produces an image native to the node the build pod ran on. On an AWS Karpenter Rack whose workload NodePool permits more than one architecture, Builds produce a multi-architecture image index. See [karpenter_arch](/configuration/rack-parameters/aws/karpenter_arch) for the Rack-level setting.

## Supported Values

| Value | Architecture | Example Instance Types |
|:------|:-------------|:-----------------------|
| `amd64` | x86/Intel/AMD | `t3.medium`, `c5.xlarge`, `m5.large` |
| `arm64` | ARM/Graviton | `t4g.medium`, `c7g.xlarge`, `m7g.large` |

Any other value is accepted when you set it and rejected when you start a Build. `convox apps params set BuildArch=x86 -a my-app` succeeds, and the next `convox build` fails immediately, before any build pod runs, with:

```
invalid BuildArch: x86, must be amd64 or arm64
```

## Use Cases
- **Pinning One App on a Multi-Architecture Rack**: Narrow a single App's Builds to one platform without changing the Rack, for example when a base image is published for only one architecture.
- **Incremental ARM Migration**: Move Apps from x86 to ARM one at a time by adding an ARM node group, setting `BuildArch=arm64`, and targeting the App to ARM workers.
- **Avoiding Cross-Architecture Emulation**: `BuildArch` avoids emulation only when the Rack also has matching build nodes, which means `build_node_enabled=true` plus a [`build_node_type`](/configuration/rack-parameters/aws/build_node_type) or build node group of that architecture. Without them the Build is emulated, which is slower than a native Build.

## Setting the Parameter
To pin an App's image to ARM:

```bash
$ convox apps params set BuildArch=arm64 -a my-app
Setting BuildArch... OK
```

To pin an App's image to x86:

```bash
$ convox apps params set BuildArch=amd64 -a my-app
Setting BuildArch... OK
```

Setting an app parameter promotes the App's current Release as a side effect, which redeploys the App's Services. The new value applies to the next Build.

To clear the pin and return the App to the Rack's build architecture, set an empty value:

```bash
$ convox apps params set BuildArch= -a my-app
Setting BuildArch... OK
```

## Viewing Current Configuration
To view the current `BuildArch` setting:

```bash
$ convox apps params -a my-app
NAME         VALUE
BuildArch    arm64
BuildCpu     500
BuildMem     1024
```

## Narrowing a Multi-Architecture Rack to One Platform
When a Build targets explicit architectures, either from the Rack or from `BuildArch`, and a base image referenced in the Dockerfile is not published for all of them, the Build fails with:

```
an image in this build is not published for the requested build architectures (amd64,arm64): set the BuildArch app parameter to an architecture the image supports (convox apps params set BuildArch=amd64 -a my-app) or use multi-arch images
```

There are two remedies. Switch the Dockerfile to a base image tag published as a multi-architecture index, or set `BuildArch` on the affected App to an architecture the image supports. Setting `BuildArch` narrows that one App's Builds to a single platform and leaves every other App on the Rack building for all architectures.

## Example: Mixed-Architecture Rack Setup

This walkthrough sets up a Rack with x86 primary nodes and adds ARM workers and ARM build nodes.

**1. Add ARM worker nodes, add ARM build nodes, and enable dedicated build nodes:**

```bash
$ convox rack params set additional_node_groups_config='[{"id":1,"type":"t4g.medium","min_size":1,"max_size":3,"label":"arm-workers","dedicated":true}]' additional_build_groups_config='[{"id":1,"type":"t4g.medium","min_size":1,"max_size":2}]' build_node_enabled=true -r my-rack
```

Set the three parameters in a single call. A Rack update holds a Terraform state lock while it applies, so a second `convox rack params set` submitted before the first finishes is rejected. Wait for `convox rack params` to return the parameter list before moving on.

Without `build_node_enabled=true` the build pod receives no architecture affinity and can schedule anywhere, so step 2 pins the image platform but does not place the Build on an ARM node.

**2. Pin the App's image to ARM:**

```bash
$ convox apps params set BuildArch=arm64 -a my-app
```

**3. Target the App to ARM workers in `convox.yml`:**

```yaml
services:
  web:
    build: .
    port: 3000
    nodeSelectorLabels:
      convox.io/label: arm-workers
```

**4. Deploy:**

```bash
$ convox deploy -a my-app
```

The build pod runs on an ARM build node because `build_node_enabled=true` and `BuildArch=arm64` are both set, the image is built for `linux/arm64`, and the resulting Processes run on the ARM worker nodes because of `nodeSelectorLabels`.

## Important Considerations

- **Requires Rack 3.25.3 or Later**: On earlier Racks the value is accepted by the CLI and discarded by the Rack, with no effect on Builds and no entry in `convox apps params`.
- **Per-App, Not Per-Service**: `BuildArch` applies to the entire App. If an App has Services targeting different architectures, split them into separate Apps.
- **Build Node Behavior Depends on `build_node_enabled`**: With `build_node_enabled=true`, the Rack must have build nodes of the specified architecture or build pods stay Pending. With `build_node_enabled=false`, the default, no placement constraint is applied and the Build proceeds under emulation when the node architecture differs.
- **Development Builds Ignore the Pin**: `convox build --development` and `convox deploy --development` do not receive the target platform, so a development Build is always native to the node it runs on.
- **External Builds Ignore the Pin**: `convox build --external` and `convox deploy --external` build the image on your own machine with the local Docker engine, which does not receive the target platform, so the image is native to that machine's architecture.
- **Provider Coverage**: The image platform pin is implemented in provider-agnostic Rack code and applies on every provider. The build node placement effect depends on `build_node_enabled`, which is an AWS Rack parameter.
- **Fluentd**: Convox system images, including Fluentd, are multi-architecture manifests and run natively on both architectures with no additional configuration.

## See Also
- [build_node_enabled](/configuration/rack-parameters/aws/build_node_enabled) for enabling dedicated build nodes, required for the placement effect
- [build_node_type](/configuration/rack-parameters/aws/build_node_type) for the architecture of the primary build node
- [karpenter_arch](/configuration/rack-parameters/aws/karpenter_arch) for the Rack-level architecture set on Karpenter Racks
- [additional_node_groups_config](/configuration/rack-parameters/aws/additional_node_groups_config) for adding node groups with different instance types
- [additional_build_groups_config](/configuration/rack-parameters/aws/additional_build_groups_config) for adding dedicated build node groups
- [BuildLabels](/configuration/app-parameters/aws/BuildLabels) for directing Builds to specific labeled node groups
- [Architecture Selection and Mixed-Architecture Racks](/configuration/scaling/karpenter#architecture-selection-and-mixed-architecture-racks) for Karpenter architecture behavior
- [Workload Placement](/configuration/scaling/workload-placement) for placement strategies
