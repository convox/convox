---
title: "Drupal"
draft: false
slug: Drupal
url: /reference/quick-apps/drupal/
---

# Quick Apps - Drupal

## Table of Contents

- [Overview](#overview)
- [Getting Started](#getting-started)
- [Installation Steps](#installation-steps)
  - [Step 1: Configure Convox Settings](#step-1-configure-convox-settings)
  - [Step 2: Configure Drupal](#step-2-configure-drupal)
  - [Step 3: Configure Database](#step-3-configure-database)
  - [Step 4: Review and Install](#step-4-review-and-install)
- [Post-Installation](#post-installation)
  - [Monitoring Installation Progress](#monitoring-installation-progress)
  - [After Installation Completion](#after-installation-completion)
- [Managing Active Apps](#managing-active-apps)
  - [Site Actions](#site-actions)
- [Site Configuration & Settings](#site-configuration--settings)
  - [Site Information](#site-information)
  - [Basic Settings](#basic-settings)
  - [Site Operations](#site-operations)
  - [Custom Configurations](#custom-configuration)
  - [Deleting a Site](#deleting-a-site)

## Overview

Quick Apps - Drupal allows you to deploy a fully functional Drupal site into your Convox rack with minimal configuration. Whether you are setting up a new Drupal installation or deploying an existing Git-based site, this streamlined process ensures a smooth setup.

For a step-by-step walkthrough of Quick Apps - Drupal, check out our **[Guided Tour](https://app.storylane.io/share/dvtwoaxuuakh)**.


> **⚠ Important Notice:**  
> To use **Quick Apps - Drupal**, your Convox **rack must be on version 3.19.7 or later**. If you are on an older version, you can see the [Updating a Rack](https://docs.convox.com/management/cli-rack-management/#updating-to-the-latest-version) information for additional guidance.  
> Additionally, you must set the rack parameter **`efs_csi_driver_enable=true`** to enable AWS EFS storage. For details, see the [EFS CSI Driver Configuration](/configuration/rack-parameters/aws/efs_csi_driver_enable) page.

## Getting Started

1. Navigate to the **Quick Apps** tab in the Convox Console.
2. Under the **Installation** tab, select **Setup** for Drupal.
3. Follow the step-by-step configuration wizard.

## Installation Steps

### Step 1: Configure Convox Settings

You will be prompted to provide the following:

- **Installation Type**  
  - *Standard Installation*: Deploys a default Drupal site with no pre-existing configurations.
  - *Git Source Installation*: Allows you to deploy from a linked GitHub repository. No special configuration files are required in the repository, but the project must follow the standard Drupal Composer structure. This includes a `composer.json` file, a `config` directory for environment settings, and a `web` directory as the document root.

- **Choose a Rack**  
  Select the Convox rack where the Drupal site will be deployed. This determines the cloud region and infrastructure used for the application.

- **App Name**  
  This defines the Convox application name within the selected rack. It is separate from the Drupal site name and is permanent once set. The application name must be unique within the rack and adhere to Convox naming rules:
  - Must be lowercase
  - No special characters except `-`
  - Can include numbers  

  The app name is how Convox identifies and manages your deployment, including routing, scaling, and service configurations.

### **Labels**  
Choose whether the application is for **Production**, **Test**, or **Dev** environments.
- *Production*: Indexed for search results.
- *Test/Dev*: Used for organization and CI/CD purposes.
- After installation, custom labels can be added.

Custom labels allow users to categorize applications based on internal organization needs. For example, they can be used to tag projects by **client name**, **development phase**, or **team ownership**, making it easier to filter and manage multiple applications.

If multiple environments (e.g., Dev, Test, and Production) are deployed for the same project, Convox automatically links them based on their shared Drupal configuration. The linked environments must have identical Drupal settings, including database schema and configuration exports, ensuring consistency across deployments. 

To simplify the deployment of identical configurations across environments, Convox provides a **Clone** functionality after site installation. This allows you to quickly replicate a Drupal installation, ensuring that Dev, Test, and Production instances remain synchronized while maintaining separate environments for development and testing.

### Step 2: Configure Drupal

- **Site Name**  
  This will be the official name of the Drupal installation.

- **Drupal Version**  
  Displays the core Drupal version that will be installed.

- **Administrator Credentials**  
  - Username  
  - Password  
  - Email address  

  These credentials will be used for logging into the Drupal site.

### Step 3: Configure Database

- **Use an Existing Database** (Optional)  
  If enabled, users can provide their database connection details in one of two ways:
  - Enter a **Database URI** with an optional prefix.  
  - Manually input individual credentials, including host, port, username, password, and database name.

  When using an existing database, Convox will not manage database lifecycle operations. This means Convox will not provide version upgrades, instance type modifications, or capacity scaling. Additionally, backup policies, high availability configurations, and maintenance schedules must be handled externally.

- **Create a New Managed Database**  
  If an existing database is not used, Convox can provision a fully managed AWS RDS-backed database in the same region as the selected rack.

  - **Database Type**: Choose between MySQL, MariaDB, or PostgreSQL.  
  - **Version Selection**:  
    - The dropdown menu includes the most commonly used versions.  
    - A custom version can be manually entered.  
    - For a full list of supported versions and deprecation timelines, refer to the official AWS documentation:  
      - [MySQL Version Management](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/MySQL.Concepts.VersionMgmt.html#MySQL.Concepts.VersionMgmt.Supported)  
      - [MariaDB Version Management](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/MariaDB.Concepts.VersionMgmt.html#MariaDB.Concepts.VersionMgmt.Supported)  
      - [PostgreSQL Version Management](https://docs.aws.amazon.com/AmazonRDS/latest/PostgreSQLReleaseNotes/postgresql-release-calendar.html#PostgreSQL.Concepts.VersionMgmt.Supported)  

  - **Instance Size**:  
    - Convox provides a set of **General Purpose** and **Memory Optimized** instance types based on common usage patterns.  
    - Users may also enter a custom instance type manually.  
    - A complete list of available RDS instance types can be found here: [AWS RDS Instance Types](https://aws.amazon.com/rds/instance-types/).  
    - **Note:** Not all instance types are available in every AWS region. Availability depends on the region where your Convox rack is deployed.

  - **Storage Capacity**: Set the initial storage size (this can be increased later if needed).

  - **Database Options**:  
    - **Deletion Protection**: Prevents accidental database deletion.  
      - If deletion protection is enabled when a Convox site is deleted, the database will persist in your AWS account. It will need to be manually removed from AWS if no longer needed.  
    - **Containerized Database**: Runs the database as a containerized service inside the Convox rack.  
      - **Best suited for cost savings** in development and testing environments.  
      - Offers lower durability and availability compared to AWS RDS-managed databases.  
      - Not recommended for production workloads due to lack of automated failover and managed backups.


### Step 4: Review and Install

- A final review page summarizes the Convox, Drupal, and database configurations before deployment.
- Click **Complete Installation** to begin deployment.

## Post-Installation

After starting an installation, you will be automatically redirected to the **Installation Job** where you can monitor the deployment process in real time. This job logs all actions being executed, including database provisioning, application setup, and service initialization.

### Monitoring Installation Progress
- You can track the installation job directly from the **Console Jobs** tab under the job labeled **Create Drupal App / <Convox App Name>**.
- At any time, you can navigate back to the installation job by selecting the **wrench icon** next to the active installation in the **Active Apps** tab of the Quick Apps Console Page.
- Alternatively, you can browse to the **Jobs** tab in the Convox Console and locate the installation job manually.

### After Installation Completion
- Once the installation job successfully completes, the new site will be listed under **Active Apps** in the Quick Apps section.
- If the installation fails or encounters an issue, details and error logs will be available in the installation job for troubleshooting.
- You can immediately access your new site using the **View Site** button in the Active Apps tab.
- Navigate to your site and log in using the **administrator credentials** you configured during installation to begin editing, adding content, and managing site configurations.

> **Next Steps:** After installation, you can configure your site, manage database settings, allocate resources, and apply labels using the **Configuration & Settings** page.

## Managing Active Apps

From the **Active Apps** tab in the Quick Apps Console, you can view all existing Drupal sites along with key details such as:
- **Labels**: Identifies the site as Production, Test, or Dev. Users can also apply **Custom Labels** for internal organization, such as tagging projects, teams, or deployment stages.
- **App Name**: The unique Convox application name.
- **Rack Name**: The Convox rack where the site is deployed.
- **Health Status**: Indicates the current state of the application.
- **Build/Job Status**: Displays active deployments, pending tasks, and any errors from previous builds. If a build or deployment fails, an error message will be shown here for troubleshooting.
- **Version**: Displays the current Drupal version in use.


### Site Actions 
Each active site includes several management options:
- **Clone**: Create a new site from an existing one.
- **View Site**: Open the live site in a new tab.
- **Configuration & Settings**: Access the site's management dashboard.
- **Active Installation Job**: During installation, you can click the **wrench icon** next to a site to navigate directly to the active installation job, allowing you to monitor progress and troubleshoot any issues.


## Site Configuration & Settings

Within the **Settings & Configuration** page, you can:
- Use the **Site Context** dropdown to switch between related environments (e.g., Dev, Test, Prod) within the same stack.
- **Rebuild Cache**: Clears the Drupal cache to apply configuration changes or resolve rendering issues.
- **View Site**: Opens the live Drupal site.

### Settings Tabs Breakdown

### **Site Information**
Displays general details about the Drupal site, including its URL, Convox application settings, database configuration, and resource allocation.

### **Basic Settings**
This section includes several subpages:

### General Settings
- **Domain Name**: Assign a custom domain to the site.  
  - Convox automatically generates a system domain for each application. See [Custom Domains](/deployment/custom-domains) for details.
- **PHP Memory Limit**: Defines the maximum amount of memory a PHP script can consume. This setting is critical for performance tuning, particularly in high-traffic Drupal environments.  
  - **Recommended Values:**
    - `128MB` – Suitable for small to medium-sized Drupal sites.
    - `256MB - 512MB` – Recommended for larger sites with complex modules or high concurrent traffic.
    - `1024MB+` – May be required for resource-intensive applications handling large media files or computational tasks.  
  - **Best Practices:**  
    - Increase memory allocation gradually based on actual site performance needs.
    - Setting limits too high may lead to inefficient resource utilization.
    - If experiencing **memory exhaustion errors**, consider adjusting this setting along with PHP execution time limits.
- **Site Name**: The display name for the Drupal site, editable after installation.
- **Trusted Domains**: Configures Drupal's `trusted_host_patterns` setting.  
  - Helps prevent HTTP Host header attacks by restricting accepted domains.

### Database Settings
- **Version**: Allows upgrading or changing the database version.  
  - **Warning**: Upgrading databases may introduce compatibility issues. Always back up your database before making changes.
- **Instance Size**: Modify the RDS instance type for performance adjustments.  
  - **Containerized Databases**: Instance size changes are not available for containerized databases since they do not run on RDS.
- **Storage Capacity**: Increase the allocated storage for the database.  

> **Note:** These options are only available for Convox-managed databases. If using an external database, configuration changes must be handled outside of Convox.

> **RDS Modification Delay:** Changes to an RDS database may take some time to fully apply, even after a build is completed. During this period:
> - The database remains in a **modifying state**, and some changes (such as instance size updates) may require a brief downtime.
> - Drupal may experience **temporary unavailability** depending on the nature of the modification.
> - Storage capacity increases are usually applied without downtime, but instance resizing or version upgrades may cause short-term disruptions.
> - It is recommended to monitor the **RDS instance status** in the AWS Console to ensure changes have fully propagated before making further modifications.

### Resource Allocations
- **vCPU**: Adjust CPU allocation (default is `0.256`).
- **Memory**: Configure memory allocation (default is `512MB`).
- **Scale**: Modify the number of application instances.  
  - **Warning**: Changing resource allocations may impact site performance and availability.

### Site Labels
- **Managed Labels**: Required Convox-managed labels (Production, Test, or Dev) determine indexing and searchability.
- **Custom Labels**: Assign additional labels for organization and filtering.  
  - **Examples**: `team-name`, `feature-branch`, `client-project`.

### Cron Settings (Timer Settings)
- **Command**: The script or command to be executed.
- **Schedule**: Defines the cron execution schedule. See [Timer Documentation](/reference/primitives/app/timer) for cron syntax details.  
  - A scheduling tool is available to simplify configuration.

> **Execution Context:** When a scheduled job runs, Convox executes a **new container/process** to run the job rather than using the existing Drupal site container. This process:
> - Has access to the **same shared file systems**, databases, and environment variables as the Drupal site.
> - Runs independently, ensuring that scheduled tasks do not interfere with the site's primary web server process.
> - Can be scaled independently, allowing for efficient handling of periodic workloads.

This means that while cron jobs have access to necessary application resources, they operate in an **isolated process** rather than directly modifying the running Drupal container.

### Site Operations

### Clone From Site
- **Source Application**: Select the site to copy from.
- **Copy Code**: Clones the entire Drupal codebase, including configurations, modules, and themes.
- **Copy Config**: Copies only the site settings, such as database configurations and resource allocations.
- **Sync Database**: Copies the source database into the selected application.

  - **Important**: The selected **source application remains unchanged**. All changes are applied **to the currently selected site**, overwriting any existing configuration.  
  - This ensures that cloned sites remain functionally consistent without affecting the original source site.

### Drupal Core Version
- Change the core version of Drupal for the site.
- **Warning**: Updating Drupal can break module compatibility. Always test in a staging environment before upgrading.

### Backups & Restore
- **Create Backup**: Generates a full snapshot of the site.
- **Restore**: Select a previous backup to roll back changes.
- **Delete Backup**: Removes a stored backup.
  - Backups are timestamped for easy identification.

### Custom Configuration

### Custom `settings.php`
- Allows modifying the `settings.php` file manually.
- Use cases include environment variable overrides and caching optimizations.
- **Changes take effect immediately**, and the service will restart automatically.

### Custom `PHP.ini`
- Modify PHP settings by pasting a custom `PHP.ini` file.
- Adjust execution limits, upload sizes, or other PHP parameters.
- **Changes apply immediately**, and the service will restart automatically.

### Deleting a Site
- Navigate to the **Delete Site** tab.
- Confirm deletion to permanently remove the application.
- **Note**: If deletion protection is enabled, the associated RDS database will persist and must be manually removed from AWS.
