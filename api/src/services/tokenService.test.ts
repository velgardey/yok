import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createToken, generateToken, hashToken, revokeToken, validateToken } from './tokenService';
import { prisma } from '../db/prisma';

vi.mock('../db/prisma', () => ({
  prisma: {
    token: {
      create: vi.fn(),
      findUnique: vi.fn(),
      updateMany: vi.fn(),
      update: vi.fn().mockResolvedValue(undefined),
    },
  },
}));

const mocked = vi.mocked(prisma.token);

describe('tokenService', () => {
  beforeEach(() => vi.clearAllMocks());

  it('generates prefixed raw token with matching hash', () => {
    const t = generateToken();
    expect(t.raw.startsWith('yok_')).toBe(true);
    expect(t.raw.length).toBeGreaterThanOrEqual(44);
    expect(t.prefix).toBe(t.raw.slice(0, 12));
    expect(t.hash).toBe(hashToken(t.raw));
    expect(t.hash).toMatch(/^[a-f0-9]{64}$/);
  });

  it('validates a good token', async () => {
    const t = generateToken();
    mocked.findUnique.mockResolvedValueOnce({
      id: 'tok1', userId: 'usr1', hash: t.hash,
      expiresAt: new Date(Date.now() + 86400_000), revokedAt: null,
    } as never);
    mocked.updateMany.mockResolvedValue({ count: 1 } as never);
    const result = await validateToken(t.raw);
    expect(result).toEqual({ userId: 'usr1', tokenId: 'tok1' });
  });

  it('rejects expired token', async () => {
    const t = generateToken();
    mocked.findUnique.mockResolvedValueOnce({
      id: 'tok1', userId: 'usr1', hash: t.hash,
      expiresAt: new Date(Date.now() - 1000), revokedAt: null,
    } as never);
    expect(await validateToken(t.raw)).toBeNull();
  });

  it('rejects revoked token', async () => {
    const t = generateToken();
    mocked.findUnique.mockResolvedValueOnce({
      id: 'tok1', userId: 'usr1', hash: t.hash,
      expiresAt: new Date(Date.now() + 86400_000), revokedAt: new Date(),
    } as never);
    expect(await validateToken(t.raw)).toBeNull();
  });

  it('rejects token whose hash mismatches', async () => {
    const t = generateToken();
    mocked.findUnique.mockResolvedValueOnce({
      id: 'tok1', userId: 'usr1', hash: hashToken('yok_other'),
      expiresAt: new Date(Date.now() + 86400_000), revokedAt: null,
    } as never);
    expect(await validateToken(t.raw)).toBeNull();
  });

  it('returns null for unknown prefix without throwing', async () => {
    mocked.findUnique.mockResolvedValueOnce(null as never);
    expect(await validateToken('yok_zzzzzzzzzzzzunknown')).toBeNull();
  });

  it('creates token with 90 day default expiry and returns raw once', async () => {
    mocked.create.mockImplementationOnce(((args: { data: { expiresAt: Date } }) =>
      Promise.resolve({ id: 'tok9', ...args.data })) as never);
    const res = await createToken('usr1', 'laptop');
    expect(res.id).toBe('tok9');
    expect(res.raw.startsWith('yok_')).toBe(true);
    const call = mocked.create.mock.calls[0][0] as { data: { expiresAt: Date } };
    const days = (call.data.expiresAt.getTime() - Date.now()) / 86_400_000;
    expect(days).toBeGreaterThan(89);
    expect(days).toBeLessThan(91);
  });

  it('revokes only own tokens', async () => {
    mocked.updateMany.mockResolvedValueOnce({ count: 1 } as never);
    expect(await revokeToken('usr1', 'tok1')).toBe(true);
    expect(mocked.updateMany).toHaveBeenCalledWith({
      where: { id: 'tok1', userId: 'usr1', revokedAt: null },
      data: { revokedAt: expect.any(Date) },
    });
  });
});
