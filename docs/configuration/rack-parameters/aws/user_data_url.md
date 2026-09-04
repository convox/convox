---
title: "user_data_url"
description: "The user_data_url AWS rack parameter points to a script downloaded and run during EC2 instance startup, defaulting to empty (no extra script)."
slug: user_data_url
url: /configuration/rack-parameters/aws/user_data_url
---

# user_data_url

## Description
The `user_data_url` parameter allows you to specify a URL to a script that will be downloaded and executed as part of the EC2 instance initialization process. This enables you to implement more complex and comprehensive customization logic for your EC2 instances beyond what can be achieved with inline commands.

The script referenced by this parameter will be downloaded and appended to the existing Convox-managed user data script, providing a powerful way to extend the instance configuration process.

## Default Value
The default value for `user_data_url` is an empty string, meaning no additional script is executed beyond the standard Convox-managed user data script.

## Use Cases
- **Complex Initialization**: Implement sophisticated instance configuration logic that would be unwieldy as inline commands.
- **Standardized Configuration**: Maintain a centralized script repository that can be referenced across multiple racks.
- **Version-Controlled Setup**: Store initialization scripts in a version control system, allowing for tracked changes and rollbacks.
- **Dynamic Configuration**: Host scripts that can be updated independently of rack parameters, enabling configuration changes without rack updates.
- **Advanced Tooling**: Install and configure complex tools, agents, or services that require multiple configuration steps.

## Setting Parameters
To set the `user_data_url` parameter, use the following command:
```bash
$ convox rack params set user_data_url=<url_endpoint> -r rackName
Updating parameters... OK
```

### Example
```bash
$ convox rack params set user_data_url=https://example.com/scripts/instance-setup.sh -r rackName
Updating parameters... OK
```

### Script Requirements
- The Convox-managed user data script already includes the shebang `#!/bin/bash`. Your custom script **must not include this line** to avoid conflicts.
- Your script **must contain only ASCII characters**. A single non-ASCII byte anywhere in it stops the entire script from running, not only the line that contains it. The node still joins the cluster and reports Ready, and the failure is reported nowhere: not in `convox`, not in the Rack update output, and not in the cloud-init log.

## Additional Information
- Ensure the script at the specified URL is accessible from the EC2 instance during initialization.
- The URL endpoint should be stable and reliable, as instance initialization will fail if the script cannot be downloaded.
- Consider using HTTPS URLs to ensure secure transmission of your script content.
- For public repositories, you can use raw content URLs such as those from GitHub, GitLab, or other public repositories.
- The script will be executed with root privileges, so ensure it contains appropriate security measures.
- To debug your user data scripts, you can SSH into an instance and examine the cloud-init logs at `/var/log/cloud-init-output.log`.
- You can extract and view the execution of your custom user data script with the following command:
  ```bash
  sudo cat /var/log/cloud-init-output.log | grep -A1000 'USER PROVIDED USER DATA SCRIPT'
  ```
  Everything after the marker is the rest of the node's bootstrap output, so read only the lines your own script produces. If your script's output does not follow this marker, or the marker itself does not appear, the script did not run. A non-ASCII character in it is the first thing to check.
- This parameter is particularly useful for complex initialization requirements that would be difficult to express using the [`user_data`](/configuration/rack-parameters/aws/user_data) parameter.
- On Rack version `3.25.6` and later, the Convox-managed user data does not start or restart kubelet before your script runs, following AWS guidance not to start or modify kubelet from launch template user data. Check any script that relied on kubelet already running.

For simpler initialization needs that can be expressed in a few commands, consider using the [`user_data`](/configuration/rack-parameters/aws/user_data) parameter instead.
