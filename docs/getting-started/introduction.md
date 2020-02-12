# Introduction

Convox is an open-source Platform as a Service that you can install on your own infrastructure.

## Supported Platforms

* Amazon Web Services
* Digital Ocean
* Google Cloud
* Microsoft Azure

## Racks

Each time you install Convox you end up with a new [Rack](../reference/primitives/rack).

A Rack is an isolated set of computing resources, network infrastructure, and storage that can contain
one or more [Apps](../reference/primitives/app).

You can use multiple Racks to isolate different environments, different customers, or different business units.

Many users have two Racks, one for staging and one for production.

You can also run a Rack on your local development workstation to develop your app in an enviornment nearly
identical to production.

## First Steps

We recommend you follow these tutorials in this order:

* [Local Development](../tutorials/local-development.md)
* [Deploying an Application](../tutorials/deploying-an-application.md)