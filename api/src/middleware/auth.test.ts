import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { NextFunction, Request, Response } from 'express';
import { requireServiceAuth, requireAuth } from './auth';
import * as users from '../services/userService';

vi.mock('../services/tokenService', () => ({
  validateToken: vi.fn(),
}));
vi.mock('../services/userService', () => ({
  findUserById: vi.fn(),
}));

import { validateToken } from '../services/tokenService';

function mockRes() {
  const res = { status: vi.fn().mockReturnThis(), json: vi.fn() } as unknown as Response;
  return res;
}
const next = vi.fn() as unknown as NextFunction;

describe('requireAuth', () => {
  beforeEach(() => vi.clearAllMocks());

  it('401s without Authorization header', async () => {
    const res = mockRes();
    await requireAuth({ headers: {} } as Request, res, next);
    expect(res.status).toHaveBeenCalledWith(401);
    expect(next).not.toHaveBeenCalled();
  });

  it('401s on invalid token', async () => {
    vi.mocked(validateToken).mockResolvedValueOnce(null);
    const res = mockRes();
    await requireAuth({ headers: { authorization: 'Bearer yok_bad' } } as Request, res, next);
    expect(res.status).toHaveBeenCalledWith(401);
  });

  it('attaches user and calls next on success', async () => {
    vi.mocked(validateToken).mockResolvedValueOnce({ userId: 'usr1', tokenId: 'tok1' });
    vi.mocked(users.findUserById).mockResolvedValueOnce({ id: 'usr1', email: 'a@b.c' } as never);
    const req = { headers: { authorization: 'Bearer yok_good' } } as Request;
    await requireAuth(req, mockRes(), next);
    expect((req as never as { user: unknown }).user).toEqual({ id: 'usr1', email: 'a@b.c' });
    expect(next).toHaveBeenCalled();
  });
});

describe('requireServiceAuth', () => {
  const handler = requireServiceAuth('yok_secret');

  it('accepts correct service token', () => {
    const req = { headers: { authorization: 'Bearer yok_secret' } } as Request;
    handler(req, mockRes(), next);
    expect(next).toHaveBeenCalled();
  });

  it('rejects wrong token', () => {
    const res = mockRes();
    handler({ headers: { authorization: 'Bearer nope' } } as Request, res, next);
    expect(res.status).toHaveBeenCalledWith(401);
  });
});
