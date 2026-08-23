import { prisma } from '../db/prisma';

const ACTIVE_STATUSES = ['PENDING', 'QUEUED', 'IN_PROGRESS'] as const;

export function createQueuedDeployment(projectId: string) {
  return prisma.deployment.create({ data: { projectId, status: 'QUEUED' } });
}

export async function promoteLatestDeployment(projectId: string, deploymentId: string) {
  await prisma.project.update({ where: { id: projectId }, data: { latestDeploymentId: deploymentId } });
}

export function markFailed(deploymentId: string) {
  return prisma.deployment.update({ where: { id: deploymentId }, data: { status: 'FAILED' } });
}

export function applyStatus(deploymentId: string, status: 'PENDING' | 'QUEUED' | 'IN_PROGRESS' | 'COMPLETED' | 'FAILED') {
  return prisma.deployment.update({ where: { id: deploymentId }, data: { status } });
}

export function findDeploymentById(id: string) {
  return prisma.deployment.findUnique({ where: { id } });
}

export function listDeployments(projectId: string) {
  return prisma.deployment.findMany({ where: { projectId }, orderBy: { createdAt: 'desc' } });
}

export async function ownedDeploymentOrError(deploymentId: string, userId: string) {
  const deployment = await findDeploymentById(deploymentId);
  if (!deployment) return { error: 'NOT_FOUND' as const };
  const project = await findProjectOwned(deployment.projectId, userId);
  if (!project) return { error: 'FORBIDDEN' as const };
  return { deployment, project };
}

function findProjectOwned(projectId: string, userId: string) {
  return prisma.project.findFirst({ where: { id: projectId, userId } });
}

export { ACTIVE_STATUSES };

export async function cancelDeployment(deploymentId: string, userId: string) {
  const found = await ownedDeploymentOrError(deploymentId, userId);
  if ('error' in found) return found;
  if (!(ACTIVE_STATUSES as readonly string[]).includes(found.deployment.status)) {
    return { error: 'NOT_CANCELLABLE' as const };
  }
  await markFailed(deploymentId);
  return { ok: true as const };
}
