package main

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	assertions "github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/jsii-runtime-go"
)

func TestStacks(t *testing.T) {
	// GIVEN
	app := awscdk.NewApp(nil)

	// WHEN
	vpcStack := NewVpcStack(app, "VpcStack", nil)
	rdsStack := NewRdsStack(app, "RdsStack", &RdsStackProps{awscdk.StackProps{}, vpcStack.Vpc})
	ecsStack := NewEcsStack(app, "EcsStack", &EcsStackProps{awscdk.StackProps{}, vpcStack.Vpc})
	applicationStack := NewApplicationStack(app, "ApplicationStack", &ApplicationStackProps{
		awscdk.StackProps{},
		ecsStack.Cluster,
	})

	// THEN
	// VPC
	vpcTemplate := assertions.Template_FromStack(vpcStack.Stack)

	vpcTemplate.HasResourceProperties(jsii.String("AWS::EC2::VPC"), map[string]interface{}{
		"CidrBlock": "10.0.0.0/21",
	})

	// 3 Subnets, 2 AZs
	vpcTemplate.ResourceCountIs(jsii.String("AWS::EC2::Subnet"), jsii.Number(3 * 2))

	// One for each AZ
	vpcTemplate.ResourceCountIs(jsii.String("AWS::EC2::NatGateway"), jsii.Number(2))

	// One in total
	vpcTemplate.ResourceCountIs(jsii.String("AWS::EC2::InternetGateway"), jsii.Number(1))

	// RDS
	rdsTemplate := assertions.Template_FromStack(rdsStack)

	rdsTemplate.ResourceCountIs(jsii.String("AWS::RDS::DBCluster"), jsii.Number(1))
	rdsTemplate.HasResourceProperties(jsii.String("AWS::RDS::DBCluster"), map[string]interface{}{
		"Engine": "aurora-postgresql",
		"EngineVersion": "10.7",
		"MasterUsername": "pgadmin",
	})

	rdsTemplate.ResourceCountIs(jsii.String("AWS::KMS::Key"), jsii.Number(1))

	rdsTemplate.ResourceCountIs(jsii.String("AWS::SecretsManager::Secret"), jsii.Number(1))

	rdsTemplate.ResourceCountIs(jsii.String("AWS::RDS::DBInstance"), jsii.Number(2))
	rdsTemplate.HasResourceProperties(jsii.String("AWS::RDS::DBInstance"), map[string]interface{}{
		"DBInstanceClass": "db.t4g.micro",
		"Engine": "aurora-postgresql",
		"EngineVersion": "10.7",
	})

	// ECS
	ecsTemplate := assertions.Template_FromStack(ecsStack.Stack)

	ecsTemplate.ResourceCountIs(jsii.String("AWS::KMS::Key"), jsii.Number(1))

	ecsTemplate.ResourceCountIs(jsii.String("AWS::Logs::LogGroup"), jsii.Number(1))

	ecsTemplate.ResourceCountIs(jsii.String("AWS::S3::Bucket"), jsii.Number(1))

	ecsTemplate.ResourceCountIs(jsii.String("AWS::IAM::InstanceProfile"), jsii.Number(1))

	ecsTemplate.ResourceCountIs(jsii.String("AWS::AutoScaling::LaunchConfiguration"), jsii.Number(1))

	ecsTemplate.ResourceCountIs(jsii.String("AWS::AutoScaling::AutoScalingGroup"), jsii.Number(1))

	ecsTemplate.ResourceCountIs(jsii.String("AWS::S3::Bucket"), jsii.Number(1))

	ecsTemplate.ResourceCountIs(jsii.String("AWS::ECS::Cluster"), jsii.Number(1))

	// Application
	applicationTemplate := assertions.Template_FromStack(applicationStack)

	applicationTemplate.ResourceCountIs(jsii.String("AWS::ElasticLoadBalancingV2::LoadBalancer"), jsii.Number(1))

	applicationTemplate.ResourceCountIs(jsii.String("AWS::ElasticLoadBalancingV2::Listener"), jsii.Number(1))
	applicationTemplate.HasResourceProperties(jsii.String("AWS::ElasticLoadBalancingV2::Listener"), map[string]interface{}{
		"Port": 3000,
		"Protocol": "HTTP",
	})

	applicationTemplate.ResourceCountIs(jsii.String("AWS::ElasticLoadBalancingV2::TargetGroup"), jsii.Number(1))
	applicationTemplate.HasResourceProperties(jsii.String("AWS::ElasticLoadBalancingV2::TargetGroup"), map[string]interface{}{
		"HealthCheckPath": "/healthcheck/",
		"HealthCheckPort": "3000",
	})

	applicationTemplate.ResourceCountIs(jsii.String("AWS::ECS::Service"), jsii.Number(1))
	applicationTemplate.HasResourceProperties(jsii.String("AWS::ECS::Service"), map[string]interface{}{
		"LaunchType": "EC2",
		"DesiredCount": 1,
		"DeploymentConfiguration": map[string]interface{}{
			"DeploymentCircuitBreaker": map[string]interface{}{
				"Rollback": true,
			},
		},
	})
}