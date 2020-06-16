# Console Rack Management

## Console vs Locally Managed Racks

When you install a Rack from your CLI, the Terraform state (and subsequently the ability to update it) is kept locally.  If you want your teammates to be able to manage, interact and update the Rack with you, you should move the Rack to be owned by an organization within the Convox Console.

> Create a free Convox account if you don't already have one, simply signup [here](https://console.convox.com/signup). We recommend using your company email address if you have one, and using your actual company name as the organization name.  Make sure you have logged in to your Convox account from the CLI by copying the login command from the web console.

## Moving your Rack to the Console

A CLI installed Rack will just have a Rack name with no organization prefix:

    $ convox racks
    NAME               PROVIDER  STATUS
    staging            gcp       running

You can transfer the Rack state to the Console by using the `rack mv` command.  Use the organization name you created in the Console as the prefix before the Rack name you wish to move to:

    $ convox rack mv staging acme/staging
    moving rack staging to acme/staging

    $ convox racks
    NAME               PROVIDER  STATUS
    acme/staging       gcp       running

The Rack will now appear in the Convox Console and your teammates with access and logged into the same organization will now see the Rack from their own CLI, and be able to interact and perform updates against the Rack from their own CLI or from the Console.

### Moving an AWS Rack

Due to an underlying issue with the way that AWS manages permissions when installing Racks, AWS-based Racks unfortunately need a further step before being able to be moved effectively. We have a longstanding bug report open with AWS to resolve this.

- First, go to your IAM console within AWS and find and note the ARN of the ConsoleRole (it will look like `arn:aws:iam::YOURACCOUNTID:role/convox-YOURORGID-ConsoleRole-0000000000`)
- On your local machine, point `kubectl` at the EKS cluster with `export KUBECONFIG=~/.kube/config.aws.RACKNAME` (replacing `RACKNAME` with the name of your Rack)
- run `kubectl edit configmap/aws-auth -n kube-system`
- Add a new item to mapRoles that looks like this

    - rolearn: arn:aws:iam::YOURACCOUNTID:role/convox-YOURORGID-ConsoleRole-0000000000
      username: convox-console
      groups:
      - system:masters

where the `rolearn` value is replaced with the full ARN of their ConsoleRole that you noted from the first step.

## Moving your Rack from the Console

You can move any Console-managed Rack back to being locally managed only with the same command:

    $ convox rack mv acme/staging staging
    moving rack acme/staging to staging

    $ convox racks
    NAME               PROVIDER  STATUS
    staging            gcp       running

Terraform state will be transferred to your local machine for exclusive management.
