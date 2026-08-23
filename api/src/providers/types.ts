export interface BuildTaskInput {
  projectId: string;
  deploymentId: string;
  gitRepoUrl: string;
  framework: string;
}

export interface ComputeProvider {
  runBuildTask(input: BuildTaskInput): Promise<void>;
}
