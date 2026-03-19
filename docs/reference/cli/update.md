---
title: "update"
slug: update
url: /reference/cli/update
---
# update

## update

Update the Convox CLI to the latest version.

### Usage
```bash
    convox update
```
### Examples
```bash
    $ convox update
    Updating to 3.23.4... OK
```

## Rack Updates

For detailed information about updating racks, please visit our [Rack Management](/management/cli-rack-management) page.

When updating across minor versions, update one minor version at a time. For example, to go from 3.21.x to 3.23.x, first update to the latest 3.22.x release, then to 3.23.x. Patch version updates within the same minor version can be applied directly.

Check your current version with `convox version`. See the [version](/reference/cli/version) command reference.

## See Also

- [CLI Rack Management](/management/cli-rack-management) for rack update best practices
- [Version](/reference/cli/version) for checking CLI and rack versions
