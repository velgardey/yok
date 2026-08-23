import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    include: ['src/**/*.test.ts'],
    env: {
      AWS_REGION: 'us-east-1',
      AWS_ACCESS_KEY_ID: 'test',
      AWS_SECRET_ACCESS_KEY: 'test',
      AWS_ECS_CLUSTER: 'test-cluster',
      AWS_ECS_TASK_DEFINITION: 'test-task',
      AWS_ECS_CONTAINER_NAME: 'builder',
      AWS_ECS_SUBNETS: 'subnet-a,subnet-b',
      AWS_ECS_SECURITY_GROUPS: 'sg-a',
    },
  },
});
