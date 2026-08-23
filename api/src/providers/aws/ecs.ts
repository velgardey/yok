import { ECSClient, RunTaskCommand, StopTaskCommand } from '@aws-sdk/client-ecs';
import type { BuildTaskHandle, BuildTaskInput, ComputeProvider } from '../types';

export interface EcsConfig {
  region: string;
  accessKeyId: string;
  secretAccessKey: string;
  cluster: string;
  taskDefinition: string;
  containerName: string;
  subnets: string[];
  securityGroups: string[];
}

export function ecsConfigFromEnv(env: NodeJS.ProcessEnv = process.env): EcsConfig {
  const required = (k: string): string => {
    const v = env[k];
    if (!v) throw new Error(`Missing ${k}`);
    return v;
  };
  return {
    region: required('AWS_REGION'),
    accessKeyId: required('AWS_ACCESS_KEY_ID'),
    secretAccessKey: required('AWS_SECRET_ACCESS_KEY'),
    cluster: required('AWS_ECS_CLUSTER'),
    taskDefinition: required('AWS_ECS_TASK_DEFINITION'),
    containerName: required('AWS_ECS_CONTAINER_NAME'),
    subnets: required('AWS_ECS_SUBNETS').split(',').map((s) => s.trim()),
    securityGroups: required('AWS_ECS_SECURITY_GROUPS').split(',').map((s) => s.trim()),
  };
}

export class EcsComputeProvider implements ComputeProvider {
  private readonly client: ECSClient;

  constructor(private readonly cfg: EcsConfig) {
    this.client = new ECSClient({
      region: cfg.region,
      credentials: { accessKeyId: cfg.accessKeyId, secretAccessKey: cfg.secretAccessKey },
    });
  }

  async runBuildTask(input: BuildTaskInput): Promise<BuildTaskHandle> {
    const command = new RunTaskCommand({
      cluster: this.cfg.cluster,
      taskDefinition: this.cfg.taskDefinition,
      launchType: 'FARGATE',
      count: 1,
      networkConfiguration: {
        awsvpcConfiguration: {
          subnets: this.cfg.subnets,
          securityGroups: this.cfg.securityGroups,
          assignPublicIp: 'ENABLED',
        },
      },
      overrides: {
        containerOverrides: [
          {
            name: this.cfg.containerName,
            environment: [
              { name: 'PROJECT_ID', value: input.projectId },
              { name: 'DEPLOYMENT_ID', value: input.deploymentId },
              { name: 'GIT_REPO_URL', value: input.gitRepoUrl },
              { name: 'FRAMEWORK', value: input.framework },
            ],
          },
        ],
      },
    });
    const response = await this.client.send(command);
    const taskArn = response.tasks?.[0]?.taskArn;
    return taskArn ? { taskArn } : {};
  }

  async stopBuildTask(taskArn: string): Promise<void> {
    await this.client.send(
      new StopTaskCommand({ cluster: this.cfg.cluster, task: taskArn, reason: 'Deployment cancelled' })
    );
  }
}
