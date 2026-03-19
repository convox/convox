---
title: "workflows"
slug: workflows
url: /reference/cli/workflows
---
# workflows

## workflows

List of workflows for the organization.

### Usage
```bash
    convox workflows
```
### Examples
```bash
    $ convox workflows
    ID                                    KIND        NAME
    55dd9440-eb98-4d9b-816f-9923ee77feff  deployment  deployweb-app
    c828b45a-070b-46ed-9c43-ddaa905ecd68  review      review-web-app
```

## workflows run <id>

Trigger workflow run for the specified branch or commit. Specified branch or commit must reside on the workflow repository.

### Usage
```bash
    convox workflows run <id>
```

### Flags

| Flag | Short | Description |
| ---- | ----- | ----------- |
| `--app` | `-a` | App name |
| `--branch` | | Git branch to run the workflow for |
| `--commit` | | Git commit to run the workflow for |
| `--title` | | Title for the workflow run |

### Examples
```bash
    $ convox workflows run 55dd9440-eb98-4d9b-816f-99230077feff --branch feat-branch --title "title"
    Successfully trigger the workflow, job id: 65a4160a-27cd-47c6-ba74-aaaaaaaa
```

## See Also

- [CI/CD Workflows](/deployment/workflows) for workflow configuration
