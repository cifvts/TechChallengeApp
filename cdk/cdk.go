package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsautoscaling"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecspatterns"
	"github.com/aws/aws-cdk-go/awscdk/v2/awskms"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsrds"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssecretsmanager"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

const (
	rdsSecret string = "pgadmin"
)

type VpcStack struct {
	Stack awscdk.Stack
	Vpc awsec2.IVpc
}

type VpcStackProps struct {
	awscdk.StackProps
}

type EcsStack struct {
	Stack awscdk.Stack
	Cluster awsecs.ICluster
}

type EcsStackProps struct {
	awscdk.StackProps
	Vpc awsec2.IVpc
}

type RdsStackProps struct {
	awscdk.StackProps
	Vpc awsec2.IVpc
}

type ApplicationStackProps struct {
	awscdk.StackProps
	Cluster awsecs.ICluster
}

func NewVpcStack(scope constructs.Construct, id string, props *VpcStackProps) VpcStack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	vpc := awsec2.NewVpc(stack, jsii.String("VPC"), &awsec2.VpcProps{
		Cidr: jsii.String("10.0.0.0/21"),
		MaxAzs: jsii.Number(2),
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
				CidrMask: jsii.Number(27),
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
		Credentials: awsrds.Credentials_FromGeneratedSecret(jsii.String(rdsSecret), &awsrds.CredentialsBaseOptions{
			EncryptionKey: kmsPostgresKey,
			SecretName: jsii.String("Postgresql pgadmin"),
		}),
		Port: jsii.Number(5432),
		StorageEncrypted: jsii.Bool(true),
		StorageEncryptionKey: kmsPostgresKey,
		InstanceProps: &awsrds.InstanceProps{
			InstanceType: awsec2.InstanceType_Of(awsec2.InstanceClass_T4G, awsec2.InstanceSize_MICRO),
			VpcSubnets: &awsec2.SubnetSelection{
				SubnetType: awsec2.SubnetType_PRIVATE_ISOLATED,
			},
			Vpc: props.Vpc,
		},
	})

	return stack
}

func NewEcsStack(scope constructs.Construct, id string, props *EcsStackProps) EcsStack {
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
		DesiredCapacity: jsii.Number(2),
		Vpc: props.Vpc,
	});

	cluster := awsecs.NewCluster(stack, jsii.String("ECSCluster"), &awsecs.ClusterProps{
		Vpc: props.Vpc,
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

	return EcsStack{stack, cluster}
}

func NewApplicationStack(scope constructs.Construct, id string, props *ApplicationStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	rdsSecret := awssecretsmanager.Secret_FromSecretNameV2(stack, jsii.String("rdsSecret"), jsii.String(rdsSecret))

	awsecspatterns.NewApplicationLoadBalancedEc2Service(stack, jsii.String("Service"), &awsecspatterns.ApplicationLoadBalancedEc2ServiceProps{
		Cluster: props.Cluster,
		MemoryLimitMiB: jsii.Number(1024),
		TaskImageOptions: &awsecspatterns.ApplicationLoadBalancedTaskImageOptions{
			Image: awsecs.ContainerImage_FromRegistry(jsii.String("servian/techchallengeapp"), &awsecs.RepositoryImageProps{}),
			Secrets: &map[string]awsecs.Secret{
				"VTT_DBUSER": awsecs.Secret_FromSecretsManager(rdsSecret, jsii.String("username")),
				"VTT_DBPASSWORD": awsecs.Secret_FromSecretsManager(rdsSecret, jsii.String("password")),
				"VTT_DBHOST": awsecs.Secret_FromSecretsManager(rdsSecret, jsii.String("host")),
				"VTT_DBPORT": awsecs.Secret_FromSecretsManager(rdsSecret, jsii.String("port")),
			},
		},
		DesiredCount: jsii.Number(1),
		CircuitBreaker: &awsecs.DeploymentCircuitBreaker{
			Rollback: jsii.Bool(true),
		},
		ListenerPort: jsii.Number(80),
	})

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

	ecsStack := NewEcsStack(app, "EcsStack", &EcsStackProps{
		awscdk.StackProps{
			Env: env(),
		},
		vpcStack.Vpc,
	})

	NewApplicationStack(app, "ApplicationStack", &ApplicationStackProps{
		awscdk.StackProps{
			Env: env(),
		},
		ecsStack.Cluster,
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
