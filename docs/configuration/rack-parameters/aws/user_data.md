---
title: "user_data"
description: "The user_data AWS rack parameter appends custom commands to the EC2 instance user data scripts run at node startup, defaulting to empty (no extra commands)."
slug: user_data
url: /configuration/rack-parameters/aws/user_data
---

# user_data

## Description
The `user_data` parameter allows you to append custom commands to the EC2 instance user data scripts managed by Convox. This enables you to execute custom initialization logic during instance startup, providing flexibility to customize your EC2 instances beyond the standard Convox configuration.

The commands specified in this parameter will be appended to the existing user data script and executed during EC2 instance initialization.

## Default Value
The default value for `user_data` is an empty string, meaning no additional commands are executed beyond the standard Convox-managed user data script.

## Use Cases
- **Custom Software Installation**: Install additional packages or software not included in the default Convox setup.
- **Configuration Management**: Apply custom configurations to system services or components.
- **Environment Setup**: Set up specific environment variables or system parameters required by your applications.
- **Monitoring Agent Installation**: Install and configure additional monitoring or logging agents.
- **Security Configuration**: Apply custom security settings or compliance requirements.

## Setting Parameters
To set the `user_data` parameter, use the following command:
```bash
$ convox rack params set user_data="<command>" -r rackName
Updating parameters... OK
```

### Example
```bash
$ convox rack params set user_data="echo 'Hello, world!' >> /tmp/init.log" -r rackName
Updating parameters... OK
```

### Script Requirements
- The Convox-managed user data script already includes the shebang `#!/bin/bash`. Your custom commands **must not include this line** to avoid conflicts.
- Your commands **must contain only ASCII characters**. A single non-ASCII byte anywhere in them stops the entire script from running, not only the line that contains it. The node still joins the cluster and reports Ready, and the failure is reported nowhere: not in `convox`, not in the Rack update output, and not in the cloud-init log.
- Multi-line commands are supported. Quote the value so your shell passes the line breaks through:
  ```bash
  $ convox rack params set user_data='echo "starting" >> /tmp/init.log
  systemctl enable my-agent' -r rackName
  Updating parameters... OK
  ```
- Commands reach the node as the characters you typed. A backslash sequence such as `\n` or `\t`, and a `${...}` sequence, are part of the script, and a `${...}` is expanded by the shell on the node.

## Additional Information
- For a script long enough to be worth keeping in version control, use the [`user_data_url`](/configuration/rack-parameters/aws/user_data_url) parameter instead.
- To debug your user data scripts, you can SSH into an instance and examine the cloud-init logs at `/var/log/cloud-init-output.log`.
- You can extract and view the execution of your custom user data script with the following command:
  ```bash
  sudo cat /var/log/cloud-init-output.log | grep -A1000 'USER PROVIDED USER DATA SCRIPT'
  ```
  Everything after the marker is the rest of the node's bootstrap output, so read only the lines your own commands produce. If your commands' output does not follow this marker, or the marker itself does not appear, the script did not run. A non-ASCII character in it is the first thing to check.
- This parameter is useful for one-time configuration during instance initialization. For ongoing configuration management, consider using a dedicated configuration management tool.
- The user data script runs with root privileges, so be cautious with the commands you specify.
- Changing `user_data` rolls the nodes in the Rack's EKS managed node groups. The commands are part of the node launch template, and EKS replaces a node group's nodes when its launch template changes. Plan the change the way you would plan a [node_type](/configuration/rack-parameters/aws/node_type) change.
- On a Rack with [`karpenter_enabled`](/configuration/rack-parameters/aws/karpenter_enabled) set to `true`, these commands run on the nodes in the Rack's EKS managed node groups only. Karpenter provisions its nodes from an `EC2NodeClass` whose `userData` carries the Convox-managed node configuration and not this script.
- On Rack version `3.25.6` and later, the Convox-managed user data does not start or restart kubelet before your commands run, following AWS guidance not to start or modify kubelet from launch template user data. Check any commands that relied on kubelet already running.

A `user_data` value containing a double quote, a line break, a backslash or a `${...}` sequence requires a `convox` CLI at `3.25.7` or newer performing the apply. Earlier versions accept the command, return `Updating parameters... OK`, and then fail the Rack update with a Terraform error, leaving the value stored and never applied.

This is not a Rack version requirement. The handling lives in the `convox` binary performing the apply, which for a Console-managed Rack is the CLI bundled in the Console deploy rather than the CLI on your machine. Run [`sudo convox update`](/reference/cli/update) to upgrade the CLI on your machine. See [A Rack update fails with a Terraform error after setting a parameter](/help/troubleshooting#a-rack-update-fails-with-a-terraform-error-after-setting-a-parameter) and [CLI Rack Management](/management/cli-rack-management).

Before `3.25.7`, a backslash sequence such as `\n` or `\t` reached the node as the character it stands for rather than as the two characters you typed, and other backslash sequences failed the Rack update. They now reach the node as typed.

For more complex initialization needs, the [`user_data_url`](/configuration/rack-parameters/aws/user_data_url) parameter provides an alternative approach by allowing you to reference a script hosted at a URL.
