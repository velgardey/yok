import { prisma } from '../db/prisma';

const ACTIVE_STATUSES = ['PENDING', 'QUEUED', 'IN_PROGRESS'] as const;
const TERMINAL_STATUSES = ['COMPLETED', 'FAILED'] as const;

export type DeploymentStatus = 'PENDING' | 'QUEUED' | 'IN_PROGRESS' | 'COMPLETED' | 'FAILED';

export function createQueuedDeployment(projectId: string) {
  return prisma.deployment.create({ data: { projectId, status: 'QUEUED' } });
}

export async function promoteLatestDeployment(projectId: string, deploymentId: string) {
  await prisma.project.update({ where: { id: projectId }, data: { latestDeploymentId: deploymentId } });
}

/**
 * Applies a status reported by the build pipeline. Terminal states are final:
 * later events for an already-completed or already-failed deployment are ignored
 * so a cancelled build cannot be resurrected by a late COMPLETED event.
 */
export async function applyStatus(deploymentId: string, status: DeploymentStatus) {
  const existing = await prisma.deployment.findUnique({ where: { id: deploymentId } });
  if (!existing) return null;
  if ((TERMINAL_STATUSES as readonly string[]).includes(existing.status)) return existing;

  return prisma.deployment.update({
    where: { id: deploymentId },
    data: {
      status,
      ...(status === 'COMPLETED' || status === 'FAILED' ? { completedAt: new Date() } : {}),
    },
  });
}

export function markFailed(deploymentId: string) {
  return applyStatus(deploymentId, 'FAILED');
}

export function setTaskArn(deploymentId: string, taskArn: string) {
  return prisma.deployment.update({ where: { id: deploymentId }, data: { taskArn } });
}

export function findDeploymentById(id: string) {
  return prisma.deployment.findUnique({ where: { id } });
}

export function listDeployments(projectId: string) {
  return prisma.deployment.findMany({ where: { projectId }, orderBy: { createdAt: 'desc' } });
}

/** Fails deployments that never reported a terminal state within `staleAfterMs`. */
export function failStaleDeployments(staleAfterMs: number) {
  const cutoff = new Date(Date.now() - staleAfterMs);
  return prisma.deployment.updateMany({
    where: { status: { in: [...ACTIVE_STATUSES] }, updatedAt: { lt: cutoff } },
    data: { status: 'FAILED', completedAt: new Date() },
  });
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

export async function cancelDeployment(deploymentId: string, userId: string) {
  const found = await ownedDeploymentOrError(deploymentId, userId);
  if (found.error) return { error: found.error };
  if (!(ACTIVE_STATUSES as readonly string[]).includes(found.deployment.status)) {
    return { error: 'NOT_CANCELLABLE' as const };
  }
  await markFailed(deploymentId);
  return { ok: true as const, taskArn: found.deployment.taskArn };
}
