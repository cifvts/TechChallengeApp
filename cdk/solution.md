# Problem analysis and solution

The scope of the document is to provide:

* a technical analysis of the problem;
* a description of the architectural and design choices;
* an explanation of the solution to specific problems.

## The problem

The main goal of this repository is to asses the abilities of the candidate's understanding of an
unknown application and to provide a solution for deploying it in a public cloud.
This should be done using IaC (Infrastructure-as-Code) tools to enable as much automation as
possible.

The challenge also assumes that the cloud account is empty and that everything necessary for the
application to run must be deployed.

## Requirements

* AWS CDK (tested with 2.34.2)

## Instruction

The code provides four stacks: `VpcStack`, `EcsStack`, `RdsStack` and `ApplicationStack`.
`VpcStack` needs to be the first to be deployed, `EcsStack` and `RdsStack` might go in parallel,
while `ApplicationStack` needs to be the last.

`cdk synth <stack>` will show the output of the stack.

`cdk deploy <stack>` will deploy the specific stack.

The AWS target account can be defined in the code or via the `CDK_DEFAULT_ACCOUNT` and
`CDK_DEFAULT_REGION` environment variables.

`go test` will execute the tests.

# Architecture

## Overview

For more information about the challenge application, refer to: [readme.md](doc/readme.md).

The challenge app is provided as a Docker image. The solution assumes the use of the AWS public
cloud and AWS CDK to deploy the infrastructure and the app.

## Architecture

Various pieces of infrastructure will need to be created in order for the app to run:

* VPC and networking;
* a PostgreSQL database;
* a service to run containers;
* the application itself;

The separation of these components is meant to:

* Provide separation between components with different life cycles (the VPC should not change very
often while the app can be deployed multiple times per day);
* Provide separation between components of different importance (app developers might not be
allowed to deploy changes to the VPC).

### VPC

The creation of a VPC (or Virtual Private Cloud) is necessary to provide a network environment in
which various pieces of the infrastructure run. A VPC in one region and multiple AZs will be enough
to connect the various services necessary.

10.0.0.0/8 can be used for the setup of the VPC, not necessarily to use it all. It can have
multiple subnets that can be used to expand different VPCs or services. The subnet assigned
to the VPC should also be segmented:

* Public subnet(s): for services that will be assigned a *public IP* (public load balancers or
EC2 instances);
* Private subnet(s): for majority of services, applications, load balancers, hosts ad so on;
* Data subnet(s): used for storage services like RDS.

This division is also meant to improve the security of the system as external actors will not be
allowed to reach the private and data network directly.

### Database

The requirement for the application is to have access to a PostgreSQL database. This can be easily
solved with AWS RDS, which allows to create specialized DB instances with various DB engines, and
PostgreSQL is supported. For the challenge, a small Graviton 2 instance will be used as it provides
a low cost and low maintenance option.

If the application(s) require, AWS RDS will allow to easily scale up by managing multi-AZs,
instances and clusters across multiple regions.

### Container service

To run a docker container in AWS, there are a few possibilities:

* A single EC2 instance running Docker and the app Docker image;
* ECS;
* EKS.

The solution using a single EC2 instance is the most simple one. It will require the creation of
the instance, a custom OS image to use (not mandatory but advised), a bootstrap script to setup it
properly and the downloading of the application docker image to run. This is also the least
automated option, and it doesn't provide scaling or high availability.

ECS can be run in various different configurations, from EC2 instances to spot instances.
A cluster running EC2 instances that can be configured at startup could be the proper option.
It also allows different applications to be deployed, allowing scalability both in the size of the
cluster and the number of tasks of any single application. It allows using CodeDeploy and
*green/blue* deployments that can rollback if the new version does not stabilize in the given time.

Applications running this way will also need more infrastructure, like an *application load
balancer (ALB)*, multiple *target groups* for the ALB, and NS names that can be used to route
network traffic from the internet to the service.

An alternative to ECS could be EKS, which uses *kubernetes* as a control plane to allocate
resources in the cluster.

The chosen solution for this challenge is to create an ECS cluster running on EC2 instances.

### Application

Using AWS CDK will also allow to deploy the application as a service in the newly created
cluster. There are possible patterns that can be used to minimize the amount of code necessary.

## Notes

### CDK

Given the time constraints, all the stacks have been put into one single file. It might be good
enough for testing purposes, but it is not recommended. In the real-case scenario, it is more
likely that different parts of the infrastructure will have different owners, and only the owner
will deploy that part of the infrastructure. The same owner might deploy different stacks with
different cycles.

AWS CDK allows to retrieve services or components deployed with different stacks. Also, using
CloudFormation exports might allow to share information between these components.

### Tests

Some tests have been provided to simply check that the infrastructure matches what we expect.
In general, this check about what can be used and what cannot be should be left to the CDK itself,
using validators, or to AWS Config.

### RDS secret

For the solution, the same secret used to create the RDS is also used by the application. This is
bad practice and should never be done. Unfortunately, because of the time constraint of the
challenge and the complexity of possible workarounds, it will not be included.

The main problem is that it is necessary to create a new user and password on the RDS. This is
done by executing SQL by accessing the DB using the admin credentials. But CDK and CF do not
provide a simple way to do this. The easy solution is to create a new user manually and then store
the credentials in a secret that would be used by the application.

If we want to automate it, one way could be to use a CF Custom Resource that will call a lambda
that will do the creation of the user. The lambda will be called before the application and will
execute the code for the creation of the user and password. Given the possibility to pass
parameters to the lambda, this could be used for multiple stacks/applications.
The lambda, once executed, should check if a secret already exists. In that case, it means it has
already created the user (unless it can be called to replace the password). Otherwise, it can create
a password, create the user in the database, and save it as a secret.