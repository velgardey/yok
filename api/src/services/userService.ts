import { prisma } from '../db/prisma';

export function findUserById(id: string) {
  return prisma.user.findUnique({ where: { id }, select: { id: true, email: true } });
}

export function countUsers() {
  return prisma.user.count();
}

export function findUserByEmail(email: string) {
  return prisma.user.findUnique({ where: { email } });
}

export function createUser(email: string) {
  return prisma.user.create({ data: { email } });
}
