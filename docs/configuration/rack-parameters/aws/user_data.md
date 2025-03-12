---
title: "user_data"
draft: false
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
```html
$ convox rack params set user_data="<command>" -r rackName
Setting parameters... OK
```

### Example
```html
$ convox rack params set user_data="echo 'Hello, world!' >> /tmp/init.log" -r rackName
Setting parameters... OK
```

## Additional Information
- The Convox-managed user data script already includes the shebang `#!/bin/bash`. Your custom commands **must not include this line** to avoid conflicts.
- Avoid complex multi-line scripts in this parameter. For more extensive scripts, consider using the [`user_data_url`](/configuration/rack-parameters/aws/user_data_url) parameter instead.
- To debug your user data scripts, you can SSH into an instance and examine the cloud-init logs at `/var/log/cloud-init-output.log`.
- You can extract and view the execution of your custom user data script with the following command:
  ```bash
  sudo cat /var/log/cloud-init-output.log | grep -A1000 'USER PROVIDED USER DATA SCRIPT' | grep -B999 '+ B64_CLUSTER_CA' | grep -v '+ B64_CLUSTER_CA'
  ```
- This parameter is useful for one-time configuration during instance initialization. For ongoing configuration management, consider using a dedicated configuration management tool.
- The user data script runs with root privileges, so be cautious with the commands you specify.
- Custom user data execution occurs after the Convox-managed setup but before the instance joins the EKS cluster, allowing you to prepare the instance environment before workloads are scheduled.

For more complex initialization needs, the [`user_data_url`](/configuration/rack-parameters/aws/user_data_url) parameter provides an alternative approach by allowing you to reference a script hosted at a URL.
