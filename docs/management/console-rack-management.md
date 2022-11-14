---
title: "Console Rack Management"
draft: false
slug: Console Rack Management
url: /management/console-rack-management
---
# Console Rack Management

## Console vs Locally Managed Racks

When you install a Rack from your CLI, the Terraform state (and subsequently the ability to update it) is kept locally.  If you want your teammates to be able to manage, interact and update the Rack with you, you should move the Rack to be owned by an organization within the Convox Console.

> Create a free Convox account if you don't already have one, simply signup [here](https://console.convox.com/signup). We recommend using your company email address if you have one, and using your actual company name as the organization name.  Make sure you have logged in to your Convox account from the CLI by copying the login command from the web console.

## Moving your Rack to the Console

A CLI installed Rack will just have a Rack name with no organization prefix:
```html
    $ convox racks
    NAME               PROVIDER  STATUS
    staging            gcp       running
```
You can transfer the Rack state to the Console by using the `rack mv` command.  Use the organization name you created in the Console as the prefix before the Rack name you wish to move to:
```html
    $ convox rack mv staging acme/staging
    moving rack staging to acme/staging

    $ convox racks
    NAME               PROVIDER  STATUS
    acme/staging       gcp       running
```
The Rack will now appear in the Convox Console and your teammates with access and logged into the same organization will now see the Rack from their own CLI, and be able to interact and perform updates against the Rack from their own CLI or from the Console.

### Moving an AWS Rack

Due to an underlying issue with the way that AWS manages permissions when installing Racks, AWS-based Racks unfortunately need a further step before being able to be moved effectively. We have a longstanding bug report open with AWS to resolve this.

- First, go to your IAM console within AWS and find and copy the ARN of the ConsoleRole.  It will look like `arn:aws:iam::YOURACCOUNTID:role/convox-YOURORGID-ConsoleRole-0000000000`.  If there is an additional `convox/` between `role/` and the `convox-YOURORGID-ConsoleRole-0000000000`, you should not include that part.
- On your local machine, point `kubectl` at the EKS cluster with `export KUBECONFIG=~/.kube/config.aws.RACKNAME` (replacing `RACKNAME` with the name of your Rack)
- run `kubectl edit configmap/aws-auth -n kube-system`
- Add a new item to mapRoles that looks like this, replacing the `rolearn` with the full ARN of their ConsoleRole that you noted from the first step.

```yaml
    - rolearn: arn:aws:iam::YOURACCOUNTID:role/convox-YOURORGID-ConsoleRole-0000000000
      username: convox-console
      groups:
      - system:masters
```

## Moving your Rack from the Console

You can move any Console-managed Rack back to being locally managed only with the same command:
```html
    $ convox rack mv acme/staging staging
    moving rack acme/staging to staging

    $ convox racks
    NAME               PROVIDER  STATUS
    staging            gcp       running
```
Terraform state will be transferred to your local machine for exclusive management.
## Access rack cluster in the EKS UI (AWS only)
You can view the Kubernetes resources deployed to your cluster with the AWS Management Console.  

### Steps:
If your rack version is > 3.6.4, you don’t need to create the cluster role and the cluster role binding.

- Create a Kubernetes clusterrolebinding that is bound to a Kubernetes clusterrole that has the necessary permissions to view the Kubernetes resources. To learn more about Kubernetes roles and role bindings, see [Using RBAC Authorization in the Kubernetes documentation](https://kubernetes.io/docs/reference/access-authn-authz/rbac/). You can apply one of the following manifests to your cluster that create a role and role binding or a cluster role and cluster role binding with the necessary Kubernetes permissions*:
```
kubectl apply -f https://s3.us-west-2.amazonaws.com/amazon-eks/docs/eks-console-full-access.yaml
```
- Make sure that the eks:AccessKubernetesApi, and other necessary IAM permissions to view Kubernetes resources, are assigned to either the user that you sign into the AWS Management Console with, or the role that you switch to once you've signed in to the console.
The following example policy includes the necessary permissions for a user or role to view Kubernetes resources for all clusters in your account. Replace `111122223333` with your account ID.
```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "eks:ListFargateProfiles",
                "eks:DescribeNodegroup",
                "eks:ListNodegroups",
                "eks:ListUpdates",
                "eks:AccessKubernetesApi",
                "eks:ListAddons",
                "eks:DescribeCluster",
                "eks:DescribeAddonVersions",
                "eks:ListClusters",
                "eks:ListIdentityProviderConfigs",
                "iam:ListRoles"
            ],
            "Resource": "*"
        },
        {
            "Effect": "Allow",
            "Action": "ssm:GetParameter",
            "Resource": "arn:aws:ssm:*:111122223333:parameter/*"
        }
    ]
}   
```
- For more information about adding users or roles to the aws-auth ConfigMap, see [Add IAM users, roles, or AWS accounts to the ConfigMap](https://docs.aws.amazon.com/eks/latest/userguide/add-user-role.html#aws-auth-users).

    1. Open the ConfigMap for editing:  `kubectl edit -n kube-system configmap/aws-auth`
    2. Add the mappings to the aws-auth ConfigMap, but don't replace any of the existing mappings. The following example adds mappings between IAM users and roles with permissions added in the first step and the Kubernetes groups created in the previous step:
    - The `my-console-viewer-role` role and the eks-console-dashboard-full-access-group.
    - The my-user user and the eks-console-dashboard-restricted-access-group.

These examples assume that you attached the IAM permissions in the first step to a role named `my-console-viewer-role` and a user named `my-user`. Replace `111122223333` with your account ID.
```yaml
apiVersion: v1
data:
  mapRoles: |
    - groups:
      - eks-console-dashboard-full-access-group
      rolearn: arn:aws:iam::111122223333:role/my-console-viewer-role
      username: my-console-viewer-role         
  mapUsers: |
    - groups:
      - eks-console-dashboard-restricted-access-group
        userarn: arn:aws:iam::111122223333:user/my-user
        username: my-user
```
### `Warning`
When you edit the `aws-auth` ConfigMap, proceed with caution, if you misconfigure it, you can lock the user out of their rack.

### Console Rack Metrics

Rack cpu and memory usages metrics support added from version `3.6.3`. So If your rack version is >= `3.6.3`, you'll be able to visualize resource consumption for your rack nodes and workloads.

![Dashboard](/images/documentation/management/console-rack-management/metrics_dashboard.png)
