---
title: "Object"
slug: object
url: /reference/primitives/app/object
---
# Object

An Object is a blob of data stored at a slash-separated path. Objects are used internally by Convox for storing build source archives, logs, and other artifacts.

Objects are managed automatically by the platform during operations like [builds](/reference/primitives/app/build) and [deployments](/deployment/deploying-changes). You do not typically interact with Objects directly.

## API Operations

The Object primitive supports the following operations through the Rack API:

| Operation  | Description                              |
| ---------- | ---------------------------------------- |
| **Store**  | Upload a blob of data at a given path    |
| **Fetch**  | Retrieve a blob of data by path          |
| **Delete** | Remove a stored object                   |
| **Exists** | Check if an object exists at a path      |
| **List**   | List objects under a given path prefix   |

Objects can be stored with an optional `Public` flag to make them accessible via a URL.
