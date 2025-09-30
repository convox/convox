---
title: "Getting Started with Convox Cloud"
draft: false
slug: getting-started
url: /cloud/getting-started
---

# Getting Started with Convox Cloud

This guide will walk you through setting up Convox Cloud and deploying your first application to a machine.  You can also follow the guided onboarding process when you first login to the [Convox Console](https://console.convox.com/signup).

## Prerequisites

Before you begin, ensure you have:

- A Convox account (sign up at [console.convox.com](https://console.convox.com/signup))
- The latest Convox CLI installed (version 3.19.0 or higher)
- A application with a `Dockerfile` ready to deploy

## Step 1: Install the Convox CLI

If you haven't installed the CLI yet, follow these instructions for your operating system:

### macOS
```bash
$ curl -L https://github.com/convox/convox/releases/latest/download/convox-macos -o /tmp/convox
$ sudo mv /tmp/convox /usr/local/bin/convox
$ sudo chmod 755 /usr/local/bin/convox
```

### Linux
```bash
$ curl -L https://github.com/convox/convox/releases/latest/download/convox-linux -o /tmp/convox
$ sudo mv /tmp/convox /usr/local/bin/convox
$ sudo chmod 755 /usr/local/bin/convox
```

Verify the installation:
```bash
$ convox version
client: 3.22.2
```

## Step 2: Login to Convox

Generate a CLI token from the Console Account page and authenticate with your Convox account:

```bash
$ convox login console.convox.com -t <CLI Token> 
Authenticating with console.convox.com... OK
```

## Step 3: Create Your First Machine

To create a machine:

1. Log into the [Convox Console](https://console.convox.com)
2. Navigate to the Cloud Machines page
3. Click the "New Machine" button
4. Configure your machine:
   - **Name**: Choose a descriptive name (e.g., "my-first-machine")
   - **Size**: Select the appropriate size (start with Small for production apps)
5. Click "Create Machine"

Once created, verify your machine is available via the CLI or within the Console page:

```bash
$ convox cloud machines
NAME               SIZE    REGION      STATUS   CREATED
my-first-machine   small   us-east-1   running  2 minutes ago
```

## Step 4: Prepare Your Application

Clone the Convox Node.js example application to get started quickly:

```bash
$ git clone https://github.com/convox-examples/nodejs.git
$ cd nodejs
```

This example repository includes everything you need to deploy to Convox Cloud:
- A simple Express.js application (`app.js`)
- Package configuration (`package.json`)
- Docker configuration (`Dockerfile`)
- Convox deployment configuration (`convox.yml`)

The application is pre-configured with:
- Express web server running on port 3000
- Minimal resource allocation (250 CPU, 512 MB memory)
- Single instance scaling
- Health check endpoint

You can deploy this application as-is to test your Convox Cloud setup, or modify it to suit your needs.

## Step 5: Create & Deploy Your Application

Create your application on the machine:

```bash
$ convox cloud apps create my-app -i my-first-machine
Creating my-app... OK
```


Deploy your application to the machine:

```bash
$ convox cloud deploy -a my-app -i my-first-machine
Packaging source... OK
Uploading source... OK
Starting build... OK
Building: .
...
Build: BABCDEFGHI
Release: RABCDEFGHI
Promoting RABCDEFGHI... OK
```

## Step 6: Access Your Application

Get the URL for your deployed application:

```bash
$ convox cloud services -a my-app -i my-first-machine
SERVICE  DOMAIN                                    PORTS
web      web.my-app.cloud.convox.com              443:3000
```

Visit the URL in your browser to see your running application.

## Step 7: View Logs

Monitor your application logs:

```bash
$ convox cloud logs -s web -a my-app -i my-first-machine
2024-01-15T10:30:00Z service/web/abc123 App listening on port 3000
2024-01-15T10:30:15Z service/web/abc123 GET / 200
```

## Step 8: Scale Your Application

Adjust the number of running processes:

```bash
$ convox cloud scale web --count 2 -a my-app -i my-first-machine
Scaling web... OK
```

Or enable autoscaling in your `convox.yml`:

```yaml
services:
  web:
    build: .
    port: 3000
    scale:
      count: 1-3
      cpu: 250
      memory: 512
      targets:
        cpu: 70
```

Then redeploy:
```bash
$ convox cloud deploy -a my-app -i my-first-machine
```

## Common Workflows

### Setting Environment Variables

```bash
$ convox cloud env set API_KEY=secret DATABASE_URL=postgres://... -a my-app -i my-first-machine
Setting API_KEY, DATABASE_URL... OK
Release: RCDEFGHIJK
```

### Running One-Off Commands

```bash
$ convox cloud run web "npm run migrate" -a my-app -i my-first-machine
Running... OK
```

### Viewing Releases

```bash
$ convox cloud releases -a my-app -i my-first-machine
ID           STATUS  BUILD        CREATED        DESCRIPTION
RCDEFGHIJK           BABCDEFGHI   1 minute ago   env add:API_KEY
RABCDEFGHI   active  BABCDEFGHI   5 minutes ago  
```

### Rolling Back

```bash
$ convox cloud releases rollback RABCDEFGHI -a my-app -i my-first-machine
Rolling back to RABCDEFGHI... OK
```

## Best Practices

1. **Start Small**: Begin with an X-Small or Small machine and scale up as needed
2. **Use Environment Variables**: Never hardcode secrets in your code
3. **Monitor Resources**: Keep an eye on CPU and memory usage to right-size your machine
4. **Enable Autoscaling**: Let Convox automatically adjust capacity based on load
5. **Regular Deployments**: Deploy frequently to catch issues early

## Troubleshooting

### Build Failures

If your build fails, check the build logs:
```bash
$ convox cloud builds logs BUILD_ID -a my-app -i my-first-machine
```

### Application Won't Start

Verify your application processes are up:
```bash
$ convox cloud ps -a my-app -i my-first-machine
```

Check service logs:
```bash
$ convox cloud logs -s my-service -a my-app -i my-first-machine
```

### Out of Resources

If you see resource errors, consider upgrading your machine size. To do this:
1. Log into the Convox Console
2. Navigate to your machine settings
3. Select a larger size
4. Apply the changes

## Next Steps

Now that you have your first application running on Convox Cloud:

- [Learn about Machine Management](/cloud/machines) - Detailed guide to machine configuration
- [Explore the CLI Reference](/cloud/cli-reference) - Complete command documentation
- [Review Sizing and Pricing](/cloud/machines/sizing-and-pricing) - Optimize your costs
- [Understand Limitations](/cloud/machines/limitations) - Know the platform constraints

## Getting Help

If you encounter issues:

- Check the [documentation](https://docs.convox.com)
- Visit the [community forum](https://community.convox.com)
- Contact support at cloud-support@convox.com