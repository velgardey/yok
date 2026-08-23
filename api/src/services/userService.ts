import { prisma } from '../db/prisma';

export function findUserById(id: string) {
  return prisma.user.findUnique({ where: { id }, select: { id: true, email: true } });
}
