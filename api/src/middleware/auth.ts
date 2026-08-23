import { timingSafeEqual } from 'crypto';
import type { NextFunction, Request, RequestHandler, Response } from 'express';
import { findUserById } from '../services/userService';
import { validateToken } from '../services/tokenService';

export interface AuthedRequest extends Request {
  user: { id: string; email: string };
}

function bearerToken(req: Request): string | null {
  const h = req.headers.authorization;
  if (!h?.startsWith('Bearer ')) return null;
  return h.slice(7);
}

export const requireAuth: RequestHandler = async (req, res, next) => {
  const raw = bearerToken(req);
  if (!raw) {
    res.status(401).json({ error: 'Missing bearer token' });
    return;
  }
  const valid = await validateToken(raw);
  if (!valid) {
    res.status(401).json({ error: 'Invalid or expired token' });
    return;
  }
  const user = await findUserById(valid.userId);
  if (!user) {
    res.status(401).json({ error: 'Invalid or expired token' });
    return;
  }
  (req as AuthedRequest).user = user;
  next();
};

export function requireServiceAuth(expectedToken: string): RequestHandler {
  return (req, res, next) => {
    const raw = bearerToken(req);
    if (!raw || !constantTimeEquals(raw, expectedToken)) {
      res.status(401).json({ error: 'Unauthorized service' });
      return;
    }
    next();
  };
}

function constantTimeEquals(a: string, b: string): boolean {
  const ba = Buffer.from(a);
  const bb = Buffer.from(b);
  return ba.length === bb.length && timingSafeEqual(ba, bb);
}
