package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsautoscaling"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awskms"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsrds"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type VpcStack struct {
	Stack awscdk.Stack
	Vpc awsec2.IVpc
}

type VpcStackProps struct {
	awscdk.StackProps
}

type EcsStackProps struct {
	awscdk.StackProps
	vpc awsec2.IVpc
}

type RdsStackProps struct {
	awscdk.StackProps
	vpc awsec2.IVpc
}

func NewVpcStack(scope constructs.Construct, id string, props *VpcStackProps) VpcStack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	vpc := awsec2.NewVpc(stack, jsii.String("VPC"), &awsec2.VpcProps{
		Cidr: jsii.String("10.0.0.0/21"),
		MaxAzs: jsii.Number(3),
		SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
			&awsec2.SubnetConfiguration{
				SubnetType: awsec2.SubnetType_PUBLIC,
				Name: jsii.String("Public"),
				CidrMask: jsii.Number(24),
			},
			&awsec2.SubnetConfiguration{
				SubnetType: awsec2.SubnetType_PRIVATE_WITH_NAT,
				Name: jsii.String("Private"),
				CidrMask: jsii.Number(24),
			},
			&awsec2.SubnetConfiguration{
				SubnetType: awsec2.SubnetType_PRIVATE_ISOLATED,
				Name: jsii.String("Data"),
				CidrMask: jsii.Number(24),
			},
		},
	})

	return VpcStack{stack, vpc}
}

func NewRdsStack(scope constructs.Construct, id string, props *RdsStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	kmsPostgresKey := awskms.NewKey(stack, jsii.String("PostgresKey"), &awskms.KeyProps{
		EnableKeyRotation: jsii.Bool(true),
	})

	awsrds.NewDatabaseCluster(stack, jsii.String("PG Database"), &awsrds.DatabaseClusterProps{
		Engine: awsrds.DatabaseClusterEngine_AuroraPostgres(&awsrds.AuroraPostgresClusterEngineProps{
			Version: awsrds.AuroraPostgresEngineVersion_VER_10_7(),
		}),
		Credentials: awsrds.Credentials_FromGeneratedSecret(jsii.String("pgadmin"), &awsrds.CredentialsBaseOptions{
			EncryptionKey: kmsPostgresKey,
			SecretName: jsii.String("Postgresql pgadmin"),
		}),
		InstanceProps: &awsrds.InstanceProps{
			InstanceType: awsec2.InstanceType_Of(awsec2.InstanceClass_T4G, awsec2.InstanceSize_MICRO),
			VpcSubnets: &awsec2.SubnetSelection{
				SubnetType: awsec2.SubnetType_PRIVATE_ISOLATED,
			},
			Vpc: props.vpc,
		},
	})

	return stack
}

func NewEcsStack(scope constructs.Construct, id string, props *EcsStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	kmsEcsExecLogKey := awskms.NewKey(stack, jsii.String("EcsExecLogKey"), &awskms.KeyProps{
		EnableKeyRotation: jsii.Bool(true),
	})

	ecsExecLogGroup := awslogs.NewLogGroup(stack, jsii.String("EcsExecLogGroup"), &awslogs.LogGroupProps{
		EncryptionKey: kmsEcsExecLogKey,
	})

	ecsExecLogBucket := awss3.NewBucket(stack, jsii.String("EcsExecLogBucket"), &awss3.BucketProps{
		EncryptionKey: kmsEcsExecLogKey,
	})

	asg := awsautoscaling.NewAutoScalingGroup(stack, jsii.String("ECSChallengeASG"), &awsautoscaling.AutoScalingGroupProps{
		InstanceType: awsec2.NewInstanceType(jsii.String("t3a.micro")),
		MachineImage: awsecs.EcsOptimizedImage_AmazonLinux2(awsecs.AmiHardwareType_STANDARD, &awsecs.EcsOptimizedImageOptions{}),
		DesiredCapacity: jsii.Number(1),
		Vpc: props.vpc,
	});

	cluster := awsecs.NewCluster(stack, jsii.String("ECSCluster"), &awsecs.ClusterProps{
		Vpc: props.vpc,
		ExecuteCommandConfiguration: &awsecs.ExecuteCommandConfiguration{
			KmsKey: kmsEcsExecLogKey,
			LogConfiguration: &awsecs.ExecuteCommandLogConfiguration{
				CloudWatchLogGroup: ecsExecLogGroup,
				CloudWatchEncryptionEnabled: jsii.Bool(true),
				S3Bucket: ecsExecLogBucket,
				S3EncryptionEnabled: jsii.Bool(true),
				S3KeyPrefix: jsii.String("exec-command-output"),
			},
			Logging: awsecs.ExecuteCommandLogging_OVERRIDE,
		},
	});

	capacity_provider := awsecs.NewAsgCapacityProvider(stack, jsii.String("AsgCapacityProvider"), &awsecs.AsgCapacityProviderProps{
		AutoScalingGroup: asg,
	});

	cluster.AddAsgCapacityProvider(capacity_provider, &awsecs.AddAutoScalingGroupCapacityOptions{})

	return stack
}

func main() {
	app := awscdk.NewApp(nil)

	vpcStack := NewVpcStack(app, "VpcStack", &VpcStackProps{
		awscdk.StackProps{
			Env: env(),
		},
	})

	NewRdsStack(app, "RdsStack", &RdsStackProps{
		awscdk.StackProps{
			Env: env(),
		},
		vpcStack.Vpc,
	})

	NewEcsStack(app, "EcsStack", &EcsStackProps{
		awscdk.StackProps{
			Env: env(),
		},
		vpcStack.Vpc,
	})

	app.Synth(nil)
}

// env determines the AWS environment (account+region) in which our stack is to
// be deployed. For more information see: https://docs.aws.amazon.com/cdk/latest/guide/environments.html
func env() *awscdk.Environment {
	// If unspecified, this stack will be "environment-agnostic".
	// Account/Region-dependent features and context lookups will not work, but a
	// single synthesized template can be deployed anywhere.
	//---------------------------------------------------------------------------
	return nil

	// Uncomment if you know exactly what account and region you want to deploy
	// the stack to. This is the recommendation for production stacks.
	//---------------------------------------------------------------------------
	// return &awscdk.Environment{
	//  Account: jsii.String("123456789012"),
	//  Region:  jsii.String("us-east-1"),
	// }

	// Uncomment to specialize this stack for the AWS Account and Region that are
	// implied by the current CLI configuration. This is recommended for dev
	// stacks.
	//---------------------------------------------------------------------------
	// return &awscdk.Environment{
	//  Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
	//  Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
	// }
}
