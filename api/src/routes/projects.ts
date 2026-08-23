import { Router } from 'express';
import { z } from 'zod';
import { requireAuth, type AuthedRequest } from '../middleware/auth';
import { createProject, findProjectById, findProjectByName, listProjects } from '../services/projectService';
import { listDeployments } from '../services/deploymentService';

const frameworkSchema = z.enum(['NEXT', 'REACT', 'VUE', 'ANGULAR', 'SVELTE', 'OTHER', 'VITE']);

export const projectsRouter: Router = Router();

projectsRouter.use(['/project', '/deploy', '/deployment', '/logs'], requireAuth);

projectsRouter.post('/project', async (req, res) => {
  const parsed = z
    .object({ name: z.string().min(1), gitRepoUrl: z.url(), framework: frameworkSchema })
    .safeParse(req.body);
  if (!parsed.success) {
    res.status(400).json({ error: parsed.error.issues.map((i) => i.message).join('; ') });
    return;
  }
  try {
    const project = await createProject((req as AuthedRequest).user.id, parsed.data);
    res.status(201).json({ status: 'success', data: { project } });
  } catch {
    res.status(500).json({ status: 'error', message: 'Failed to create project' });
  }
});

projectsRouter.get('/project', async (req, res) => {
  const projects = await listProjects((req as AuthedRequest).user.id);
  res.status(200).json({ status: 'success', data: { projects } });
});

projectsRouter.get('/project/check', async (req, res) => {
  const parsed = z.object({ name: z.string().min(1) }).safeParse(req.query);
  if (!parsed.success) {
    res.status(400).json({ status: 'error', message: 'name query parameter required' });
    return;
  }
  const project = await findProjectByName(parsed.data.name);
  if (project && project.userId !== (req as AuthedRequest).user.id) {
    res.status(200).json({ status: 'success', data: { exists: false } });
    return;
  }
  res.status(200).json({ status: 'success', data: project ? { exists: true, project } : { exists: false } });
});

projectsRouter.get('/project/:id/deployments', async (req, res) => {
  const project = await findProjectById(req.params.id);
  if (!project || project.userId !== (req as unknown as AuthedRequest).user.id) {
    res.status(404).json({ status: 'error', message: 'Project not found' });
    return;
  }
  res.status(200).json({ status: 'success', data: { deployments: await listDeployments(project.id) } });
});

projectsRouter.get('/project/:id', async (req, res) => {
  const project = await findProjectById(req.params.id);
  if (!project || project.userId !== (req as unknown as AuthedRequest).user.id) {
    res.status(404).json({ status: 'error', message: 'Project not found' });
    return;
  }
  res.status(200).json({ status: 'success', data: { project } });
});
