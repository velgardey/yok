import { Router } from 'express';
import type { AppConfig } from '../config';
import { requireAuth, type AuthedRequest } from '../middleware/auth';
import { countUsers, createUser, findUserByEmail } from '../services/userService';
import { createToken, revokeToken } from '../services/tokenService';
import { prisma } from '../db/prisma';

export function authRouter(config: AppConfig): Router {
  const router = Router();

  router.post('/auth/bootstrap', async (req, res) => {
    const secret = req.headers['x-bootstrap-secret'];
    const usersExist = (await countUsers()) > 0;
    if (usersExist && (!config.bootstrapSecret || secret !== config.bootstrapSecret)) {
      res.status(403).json({ status: 'error', message: 'Bootstrap already completed' });
      return;
    }
    const email = typeof req.body?.email === 'string' ? req.body.email : '';
    if (!/.+@.+\..+/.test(email)) {
      res.status(400).json({ status: 'error', message: 'Valid email required' });
      return;
    }
    const user = (await findUserByEmail(email)) ?? (await createUser(email));
    const { id, raw } = await createToken(user.id, 'bootstrap');
    res.status(201).json({
      status: 'success',
      data: { userId: user.id, tokenId: id, token: raw, warning: 'Save this token now; it will not be shown again.' },
    });
  });

  router.post('/auth/tokens', requireAuth, async (req, res) => {
    const name = typeof req.body?.name === 'string' && req.body.name.trim() ? req.body.name.trim() : 'unnamed';
    const days = typeof req.body?.expiresInDays === 'number' ? Math.min(Math.max(req.body.expiresInDays, 1), 365) : undefined;
    const { id, raw } = await createToken((req as AuthedRequest).user.id, name, days);
    res.status(201).json({
      status: 'success',
      data: { tokenId: id, token: raw, warning: 'Save this token now; it will not be shown again.' },
    });
  });

  router.delete('/auth/tokens/:id', requireAuth, async (req, res) => {
    const revoked = await revokeToken((req as AuthedRequest).user.id, req.params.id);
    if (!revoked) {
      res.status(404).json({ status: 'error', message: 'Token not found' });
      return;
    }
    res.status(200).json({ status: 'success', message: 'Token revoked' });
  });

  router.get('/auth/me', requireAuth, (req, res) => {
    res.status(200).json({ status: 'success', data: { user: (req as AuthedRequest).user } });
  });

  router.get('/auth/tokens', requireAuth, async (req, res) => {
    const tokens = await prisma.token.findMany({
      where: { userId: (req as AuthedRequest).user.id, revokedAt: null },
      select: { id: true, name: true, prefix: true, createdAt: true, expiresAt: true, lastUsedAt: true },
      orderBy: { createdAt: 'desc' },
    });
    res.status(200).json({ status: 'success', data: { tokens } });
  });

  return router;
}
