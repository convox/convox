---
title: "Machine Management"
description: "The convox cloud machines command lists all machines in your organization; create, update, and delete machines from the Convox Console."
slug: machines
url: /cloud/cli-reference/machines
---

# Machine Management

### machines

List all machines in your organization.

```bash
$ convox cloud machines
NAME         SIZE    REGION      STATUS   CREATED
production   large   us-east-1   running  2 weeks ago
staging      small   us-west-2   running  1 month ago
```

**Note**: To create, update, or delete machines, use the Convox Console. Log in at [console.convox.com](https://console.convox.com) and navigate to the Cloud Machines page.
