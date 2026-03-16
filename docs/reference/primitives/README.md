---
title: "Primitives"
slug: primitives
url: /reference/primitives
---
# Primitives

Primitives are the building blocks available to build an [App](/reference/primitives/app) on Convox. Apps are deployed and run on your [Rack](/reference/primitives/rack), the platform overlaid on your cloud infrastructure.

Primitives can be composed to provide useful functionality that lets you
focus on the things that make your [App](/reference/primitives/app) unique.

## App Primitives

| Primitive                   | Description                                                                                  |
|:----------------------------|:---------------------------------------------------------------------------------------------|
| [Balancer](/reference/primitives/app/balancer) | Custom TCP load balancers in front of a [Service](/reference/primitives/app/service)                            |
| [Build](/reference/primitives/app/build)       | Compiled version of a codebase                                                               |
| [Object](/reference/primitives/app/object)     | Blob/file storage                                                                            |
| [Process](/reference/primitives/app/process)   | Running containers created by running a command on a [Release](/reference/primitives/app/release)                 |
| [Release](/reference/primitives/app/release)   | Units of deployment consisting of a [Build](/reference/primitives/app/build) and a set of environment variables |
| [Resource](/reference/primitives/app/resource) | Network-accessible external services (e.g. Postgres)                                         |
| [Service](/reference/primitives/app/service)   | Horizontally-scalable collections of durable [Processes](/reference/primitives/app/process)                     |
| [Timer](/reference/primitives/app/timer)       | Runs one-off [Processes](/reference/primitives/app/process) on a scheduled interval                             |

## Rack Primitives

| Primitive                    | Description                                                         |
|:-----------------------------|:--------------------------------------------------------------------|
| [Instance](/reference/primitives/rack/instance) | Node that provides capacity for running [Processes](/reference/primitives/app/process) |
| [Registry](/reference/primitives/rack/registry) | External image repository                                           |