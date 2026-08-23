export interface BuildTaskInput {
  projectId: string;
  deploymentId: string;
  gitRepoUrl: string;
  framework: string;
}

export interface BuildTaskHandle {
  taskArn?: string;
}

export interface ComputeProvider {
  runBuildTask(input: BuildTaskInput): Promise<BuildTaskHandle>;
  stopBuildTask(taskArn: string): Promise<void>;
}
