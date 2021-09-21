---
title: "apps"
draft: false
slug: apps
url: /reference/cli/apps
---
# apps

## apps

List apps

### Usage
```html
    convox apps
```
### Examples
```html
    $ convox apps
    APP          STATUS   RELEASE
    myapp        running  RABCDEFGHI
    myapp2       running  RIHGFEDCBA
```
## apps cancel

Cancel an app update

### Usage
```html
    convox apps cancel [app]
```
### Examples
```html
    $ convox apps cancel
    Cancelling deployment of myapp... OK
```
## apps create

Create an app

### Usage
```html
    convox apps create [app]
```
### Examples
```html
    $ convox apps create myapp
    Creating myapp... OK
```
## apps delete

Delete an app

### Usage
```html
    convox apps delete <app>
```
### Examples
```html
    $ convox apps delete myapp
```
## apps export

Export an app

### Usage
```html
    convox apps export [app]
```
### Examples
```html
    $ convox apps export --file myapp.tgz
    Exporting app myapp... OK
    Exporting env... OK
    Exporting build BABCDEFGHI... OK
    Exporting resource database... OK
    Packaging export... OK
```
## apps import

Import an app

### Usage
```html
    convox apps import [app]
```
### Examples
```html
    $ convox apps import myapp2 --file myapp.tgz
    Creating app myapp2... OK
    Importing build... OK, RIHGFEDCBA
    Importing env... OK, RJIHGFEDCB
    Promoting RJIHGFEDCB... OK
    Importing resource database... OK
```
## apps info

Get information about an app

### Usage
```html
    convox apps info [app]
```
### Examples
```html
    $ convox apps info
    Name        myapp
    Status      running
    Generation  2
    Locked      false
    Release     RABCDEFGHI
```
## apps lock

Enable termination protection

### Usage
```html
    convox apps lock [app]
```
### Examples
```html
    $ convox apps lock
    Locking myapp... OK
```
## apps unlock

Disable termination protection

### Usage
```html
    convox apps unlock [app]
```
### Examples
```html
    $ convox apps unlock
    Unlocking myapp... OK
```