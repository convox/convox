---
title: "Example Apps"
draft: false
slug: Example Apps
url: /example-apps
---
# Example Apps

Convox provides a comprehensive collection of example applications demonstrating how to deploy various languages, frameworks, and popular open-source applications. These examples showcase the simplicity of going from a fresh application to a production-ready deployment on Convox.

All example applications are available in our [GitHub organization](https://github.com/convox-examples/), with each repository containing:
- Complete `convox.yml` configuration
- Optimized Dockerfiles
- Step-by-step deployment instructions
- Environment variable documentation
- Best practices for production deployments

Whether you're deploying to Convox Cloud for a fully-managed experience or to your own Convox Rack for complete infrastructure control, these examples will help you get started quickly.

## Languages

Deploy applications built with your preferred programming language. Each example demonstrates language-specific best practices for containerization and deployment.

### [Node.js](https://github.com/convox-examples/nodejs)
A Node.js application example featuring Express.js, demonstrating proper package management, multi-stage Docker builds, and health check configuration. Perfect for APIs, web services, and real-time applications using JavaScript or TypeScript.

### [Ruby](https://github.com/convox-examples/rails)
Ruby application deployment showcasing the Rails framework with Active Record, Redis integration, and Sidekiq for background jobs. Demonstrates Ruby-specific optimizations and bundler best practices.

### [PHP](https://github.com/convox-examples/php)
PHP application deployment using Apache or Nginx, with examples for both traditional PHP applications and modern frameworks. Includes PHP-FPM configuration and session handling best practices.

### [Deno](https://github.com/convox-examples/deno)
Modern Deno runtime deployment example demonstrating TypeScript-first development, built-in security features, and simplified dependency management without node_modules.

## Backend Frameworks

Production-ready backend framework examples with database integration, API development, and business logic implementation.

### [Django](https://github.com/convox-examples/django)
Full-featured Python Django application with PostgreSQL integration, static file handling, and migration management. Includes configuration for both development and production environments with proper secret management, Django admin setup, and REST API capabilities.

### [Ruby on Rails](https://github.com/convox-examples/rails)
Complete Rails application setup including Active Record with PostgreSQL, Redis for caching/Action Cable, background job processing with Sidekiq, and asset pipeline configuration. Demonstrates zero-downtime deployments and database migration strategies.

### [.NET Core](https://github.com/convox-examples/dotnet-core)
Cross-platform .NET Core framework deployment showcasing ASP.NET Core web APIs and MVC applications built with C#. Features multi-stage builds for optimized container sizes, Entity Framework Core integration, and configuration management through environment variables.

## Frontend Frameworks

Modern frontend framework examples optimized for client-side applications, featuring hot-reloading for development and production-ready builds.

### [Next.js](https://github.com/convox-examples/nextjs)
Full-stack React framework deployment with server-side rendering (SSR), static site generation (SSG), and API routes. Includes environment-specific builds, CDN-ready static asset configuration, and integration with backend services.

### [Svelte](https://github.com/convox-examples/Svelte)
Lightweight Svelte application deployment showcasing the framework's compiled approach to building user interfaces. Features SvelteKit for full-stack capabilities, optimized production builds, and minimal runtime overhead.

## Web Servers

Examples of deploying traditional web servers and static content.

### [Apache httpd](https://github.com/convox-examples/httpd)
Apache HTTP Server configuration for serving static websites, reverse proxy setups, and traditional web hosting scenarios. Includes custom configuration files, SSL setup, and mod_rewrite examples.

## Hosted Applications

Deploy popular open-source applications with production-ready configurations, including database setup, authentication, and scaling recommendations.

### [n8n Workflow Automation](https://github.com/convox-examples/n8n)
Complete deployment of the n8n workflow automation platform featuring:
- PostgreSQL database integration for persistent storage
- Webhook support with automatic SSL certificates
- SMTP configuration for email notifications
- User authentication and management
- Scaling strategies for queue mode with Redis
- Backup and restore procedures

Perfect for teams looking to self-host their automation workflows with 400+ service integrations.

## Getting Started

Each example repository includes:

1. **README.md** - Comprehensive documentation including:
   - Quick start instructions for both Convox Cloud and Rack deployments
   - Required and optional environment variables
   - Database configuration options
   - Scaling recommendations
   - Troubleshooting guides

2. **convox.yml** - Production-ready Convox configuration with:
   - Service definitions
   - Resource declarations
   - Health check endpoints
   - Scaling parameters

3. **Dockerfile** - Optimized container configuration using:
   - Multi-stage builds for smaller images
   - Security best practices
   - Proper signal handling for graceful shutdowns

## Deployment Process

All examples follow the same basic deployment pattern:

### Convox Cloud
```bash
# Create your app
convox cloud apps create myapp -i machine-name

# Set any required environment variables
convox cloud env set KEY=value -a myapp -i machine-name

# Deploy
convox cloud deploy -a myapp -i machine-name
```

### Convox Rack
```bash
# Create your app
convox apps create myapp

# Set any required environment variables
convox env set KEY=value -a myapp

# Deploy
convox deploy -a myapp
```

## Contributing

We welcome contributions! If you have an example application you'd like to share:

1. Follow the existing repository structure
2. Include comprehensive documentation
3. Add production-ready configuration
4. Submit an issue or pull request to the relevant repository

## Need a Specific Example?

Can't find an example for your stack? We're constantly adding new examples based on community needs. Request a specific example by:

- Opening an issue in the [convox-examples](https://github.com/convox-examples/) organization
- Reaching out through the [Convox Community Forum](https://community.convox.com)
- Contacting support at support@convox.com

## Resources

- [Convox Documentation](https://docs.convox.com)
- [convox.yml Reference](/configuration/convox-yml)
- [Dockerfile Best Practices](/configuration/dockerfile)
- [Deployment Guide](/deployment)
- [Environment Variables](/configuration/environment)