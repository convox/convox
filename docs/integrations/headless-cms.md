---
title: "Headless CMS"
slug: headless-cms
url: /integrations/headless-cms
---
# Headless CMS Integrations

Convox integrates with headless Content Management Systems (CMS) to connect your content publishing workflows with your deployment processes. Headless CMS platforms separate content management from the presentation layer, allowing content creators to trigger deployments directly from their CMS interface without switching contexts or manually triggering deployments from the Convox Console.

## Contentful

Contentful is a headless CMS built with an API-first approach. Using Contentful, you can create, organize, and edit content through their web interface, then deploy it to your applications using APIs.

The Convox Contentful integration enables workflow triggering between your Contentful space and your Convox applications. When content is updated in Contentful, you can deploy your applications directly from the Contentful interface.

### Before You Begin

To integrate Contentful with your Convox environment, ensure you have:

* A Contentful account and an active Contentful space with content types defined
* A Convox Rack with at least one application
* A [Deployment Workflow](/deployment/workflows) configured for your application in the Convox Console

### Installation and Configuration

#### Step 1: Install the Convox App in Contentful

1. Log in to your Contentful account and navigate to your space
2. Click on **Apps** in the top navigation menu
3. Select **Marketplace** to browse available applications
4. Search for "Convox" and select the Convox app

![Contentful Marketplace - Convox App](/images/documentation/integrations/headless-cms/contentful/contentful-marketplace.png)

5. Click **Install** to add the Convox app to your Contentful space

#### Step 2: Create a Deploy Key in the Convox Console

1. Log in to the [Convox Console](https://console.convox.com)
2. Navigate to **Settings** from the left sidebar
3. Go to the **Deploy Keys** section
4. Click **Create** to generate a new deploy key
5. Give your deploy key a descriptive name (e.g., "Contentful Integration")
6. Copy the generated deploy key value - you will need this in the next step

![Convox Deploy Key Creation](/images/documentation/integrations/headless-cms/contentful/convox-deploy-key.png)

#### Step 3: Configure the Convox App in Contentful

1. In Contentful, navigate to **Apps** > **Installed apps**
2. Find and select the Convox app you installed
3. In the configuration screen, paste your Convox Deploy Key into the provided field
4. Click **Add Workflows** to connect to the Convox API

![Convox App Configuration](/images/documentation/integrations/headless-cms/contentful/contentful-app-config.png)

If the connection is successful, you should see your Convox Workflows appear in the list. These are the workflows that have been configured in your Convox Console.

#### Step 4: Configure Content Type Access

In the same configuration screen:

1. Under **Content Types**, select which Contentful content types should display the Convox functionality in the sidebar
2. Click **Save** to apply your changes

### Using the Integration

Once configured, the Convox integration can be used directly from the Contentful interface:

1. Navigate to the **Content** section in Contentful
2. Select any entry of a content type you configured for Convox access
3. In the right sidebar, you'll see the Convox section with a dropdown of available workflows
4. Select the desired workflow from the dropdown
5. Click **Run Workflow** to trigger a deployment

![Running a Workflow from Contentful](/images/documentation/integrations/headless-cms/contentful/run-workflow.png)

The workflow will execute in Convox, and you'll see a status indicator showing whether the deployment was successful.

### Troubleshooting

If you encounter issues with the Contentful integration, check the following:

* Ensure your Deploy Key is valid and has the correct permissions
* Verify that you have properly configured Deployment Workflows in the Convox Console
* Check that you've selected the correct content types for the Convox app in Contentful
* Confirm your Convox Rack and application are running correctly

### Best Practices

* Create separate Deploy Keys for different integrations to maintain better security and audit trails
* Consider using specific Deployment Workflows for content-only updates versus code deployments
* Configure the integration only for content types that should trigger deployments

### More Resources

* [Contentful Documentation](https://www.contentful.com/developers/docs/)
* [Convox CI/CD Workflows](/deployment/workflows)
* [Convox Deploy Keys](/management/deploy-keys)
