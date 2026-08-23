import { Router } from 'express';
import { requireServiceAuth } from '../middleware/auth';
import { findProjectBySlug } from '../services/projectService';

export function resolveRouter(proxyToken: string): Router {
  const router = Router();

  router.get('/resolve/:slug', requireServiceAuth(proxyToken), async (req, res) => {
    const slug = req.params.slug;
    if (!slug) {
      res.status(400).json({ error: 'slug required' });
      return;
    }
    try {
      const project = await findProjectBySlug(slug);
      if (!project?.latestDeploymentId) {
        res.status(404).json({ error: 'Project or latest deployment not found' });
        return;
      }
      res.status(200).json({ deploymentId: project.latestDeploymentId });
    } catch {
      res.status(500).json({ error: 'Failed to resolve slug' });
    }
  });

  return router;
}
