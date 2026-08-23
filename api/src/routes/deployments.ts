import { Router } from 'express';
import { z } from 'zod';
import type { AppConfig } from '../config';
import { getComputeProvider } from '../providers/factory';
import { requireAuth, type AuthedRequest } from '../middleware/auth';
import { findProjectById } from '../services/projectService';
import {
  cancelDeployment,
  createQueuedDeployment,
  markFailed,
  ownedDeploymentOrError,
  promoteLatestDeployment,
} from '../services/deploymentService';
import { LogStore } from '../logs/clickhouse';

const uuidSchema = z.uuid();

export function deploymentsRouter(config: AppConfig, logStore: LogStore): Router {
  const router = Router();
  const compute = getComputeProvider(config);

  router.post('/deploy', requireAuth, async (req, res) => {
    const parsed = z.object({ projectId: uuidSchema }).safeParse(req.body);
    if (!parsed.success) {
      res.status(400).json({ error: parsed.error.issues.map((i) => i.message).join('; ') });
      return;
    }
    const project = await findProjectById(parsed.data.projectId);
    if (!project || project.userId !== (req as AuthedRequest).user.id) {
      res.status(404).json({ error: 'Project not found' });
      return;
    }

    const deployment = await createQueuedDeployment(project.id);
    await promoteLatestDeployment(project.id, deployment.id);

    try {
      await compute.runBuildTask({
        projectId: project.id,
        deploymentId: deployment.id,
        gitRepoUrl: project.gitRepoUrl,
        framework: project.framework,
      });
      res.status(202).json({
        status: 'success',
        data: {
          deploymentId: deployment.id,
          deploymentUrl: `https://${deployment.id}.${config.siteDomain}/`,
        },
      });
    } catch (error) {
      console.error('Error running build task:', error);
      await markFailed(deployment.id);
      res.status(500).json({ status: 'error', message: 'Failed to deploy project' });
    }
  });

  router.get('/logs/:id', requireAuth, async (req, res) => {
    const parsed = uuidSchema.safeParse(req.params.id);
    if (!parsed.success) {
      res.status(400).json({ error: 'Invalid deployment id' });
      return;
    }
    const owned = await ownedDeploymentOrError(parsed.data, (req as AuthedRequest).user.id);
    if ('error' in owned) {
      res.status(owned.error === 'FORBIDDEN' ? 403 : 404).json({ error: 'Deployment not found' });
      return;
    }
    res.status(200).json({ status: 'success', data: { logs: await logStore.listByDeployment(parsed.data) } });
  });

  router.get('/deployment/:id', requireAuth, async (req, res) => {
    const parsed = uuidSchema.safeParse(req.params.id);
    if (!parsed.success) {
      res.status(400).json({ status: 'error', message: 'Invalid deployment id' });
      return;
    }
    const owned = await ownedDeploymentOrError(parsed.data, (req as AuthedRequest).user.id);
    if ('error' in owned) {
      res.status(owned.error === 'FORBIDDEN' ? 403 : 404).json({ status: 'error', message: 'Deployment not found' });
      return;
    }
    res.status(200).json({ status: 'success', data: { deployment: owned.deployment } });
  });

  router.post('/deployment/:id/cancel', requireAuth, async (req, res) => {
    const parsed = uuidSchema.safeParse(req.params.id);
    if (!parsed.success) {
      res.status(400).json({ status: 'error', message: 'Invalid deployment id' });
      return;
    }
    const result = await cancelDeployment(parsed.data, (req as AuthedRequest).user.id);
    if ('error' in result) {
      const status = result.error === 'NOT_FOUND' ? 404 : result.error === 'FORBIDDEN' ? 403 : 400;
      res.status(status).json({ status: 'error', message: `Cannot cancel deployment (${result.error})` });
      return;
    }
    res.status(200).json({ status: 'success', message: 'Deployment cancelled successfully' });
  });

  return router;
}
