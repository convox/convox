# instances

## instances

List instances

### Usage

    convox instances

### Examples

    $ convox instances
    ID                   STATUS  STARTED       PS  CPU     MEM     PUBLIC          PRIVATE
    i-029382969778a743a  active  2 months ago  3   18.75%  45.08%  32.207.218.250  10.0.2.39
    i-06d0eaf588c96ee5a  active  2 months ago  2   18.75%  32.64%  52.208.102.198  10.0.2.17
    i-0a69dd90d3b542c3a  active  2 months ago  3   21.88%  58.13%  52.160.141.135  10.0.1.151
    i-0cbaa6d2dd1d094ca  active  2 months ago  5   37.50%  77.72%  1.226.241.132   10.0.3.45
    i-0d4493dded1fa9aea  active  2 months ago  5   50.00%  97.91%  52.144.245.283  10.0.1.56

## instances keyroll

Roll ssh key on instances

### Usage

    convox instances keyroll

### Examples

    $ convox instances keyroll
    Rolling instance key...
    ...
    ...
    2020-01-31T15:44:37Z system/cloudformation aws/cfm custom2 UPDATE_IN_PROGRESS Instances Received SUCCESS signal with UniqueId i-0f5fb7f262fd2c50a
    2020-01-31T15:44:38Z system/cloudformation aws/cfm custom2 UPDATE_IN_PROGRESS Instances Terminating instance(s) [i-075f25eed26ea9cea]; replacing with 1 new instance(s).
    OK


## instances ssh

Run a shell on an instance

### Usage

    convox instances ssh <instance_id>

### Examples

    $ convox instances ssh i-029382969778a743a

    __|  __|  __|
    _|  (   \__ \   Amazon Linux 2 (ECS Optimized)
    ____|\___|____/

    For documentation, visit http://aws.amazon.com/documentation/ecs
    17 package(s) needed for security, out of 46 available
    Run "sudo yum update" to apply all updates.
    [ec2-user@ip-10-0-2-39 ~]$

## instances terminate

Terminate an instance

### Usage

    convox instances terminate <instance_id>

### Examples

    $ convox instances terminate i-029382969778a743a
    Terminating instance... OK