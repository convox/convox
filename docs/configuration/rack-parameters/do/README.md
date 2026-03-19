---
title: "Digital Ocean Rack Parameters"
slug: do-rack-parameters
url: /configuration/rack-parameters/do
---
# Digital Ocean Rack Parameters

The following parameters are available for configuring your Convox rack on Digital Ocean. These parameters allow you to customize and optimize the behavior of your applications and services running on the Digital Ocean platform.

> Some parameters can only be set during rack installation and cannot be changed afterwards. These include `region`. See individual parameter pages for details.

## Parameters

| Parameter                            | Description                                                              |
|:-------------------------------------|:-------------------------------------------------------------------------|
| [cert_duration](/configuration/rack-parameters/do/cert_duration)         | Certificate renewal period.                                               |
| [docker_hub_password](/configuration/rack-parameters/do/docker_hub_password) | Docker Hub access token for authenticated image pulls. |
| [docker_hub_username](/configuration/rack-parameters/do/docker_hub_username) | Docker Hub username for authenticated image pulls. |
| [node_type](/configuration/rack-parameters/do/node_type)                 | Specifies the node instance type.                                         |
| [region](/configuration/rack-parameters/do/region)                       | Specifies the Digital Ocean region for the rack.                          |
| [registry_disk](/configuration/rack-parameters/do/registry_disk)         | Specifies the size of the registry disk.                                  |
| [syslog](/configuration/rack-parameters/do/syslog)                       | Specifies the endpoint to forward logs to a syslog server.                |
