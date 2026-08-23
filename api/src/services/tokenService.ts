import { createHash, randomBytes, timingSafeEqual } from 'crypto';
import { prisma } from '../db/prisma';

const TOKEN_LIFETIME_DAYS = 90;

export function hashToken(raw: string): string {
  return createHash('sha256').update(raw).digest('hex');
}

export function generateToken(): { raw: string; prefix: string; hash: string } {
  const raw = `yok_${randomBytes(32).toString('base64url')}`;
  return { raw, prefix: raw.slice(0, 12), hash: hashToken(raw) };
}

export async function validateToken(raw: string): Promise<{ userId: string; tokenId: string } | null> {
  if (!raw.startsWith('yok_') || raw.length < 20) return null;
  const token = await prisma.token.findUnique({ where: { prefix: raw.slice(0, 12) } });
  if (!token) return null;

  const incoming = Buffer.from(hashToken(raw), 'hex');
  const stored = Buffer.from(token.hash, 'hex');
  if (incoming.length !== stored.length || !timingSafeEqual(incoming, stored)) return null;

  const now = new Date();
  if (token.revokedAt || token.expiresAt <= now) return null;

  void prisma.token
    .update({ where: { id: token.id }, data: { lastUsedAt: now } })
    .catch(() => undefined);

  return { userId: token.userId, tokenId: token.id };
}

export async function createToken(
  userId: string,
  name: string,
  days: number = TOKEN_LIFETIME_DAYS
): Promise<{ id: string; raw: string }> {
  const { raw, prefix, hash } = generateToken();
  const expiresAt = new Date(Date.now() + days * 86_400_000);
  const created = await prisma.token.create({
    data: { userId, name, prefix, hash, expiresAt },
  });
  return { id: created.id, raw };
}

export async function revokeToken(userId: string, tokenId: string): Promise<boolean> {
  const res = await prisma.token.updateMany({
    where: { id: tokenId, userId, revokedAt: null },
    data: { revokedAt: new Date() },
  });
  return res.count > 0;
}
