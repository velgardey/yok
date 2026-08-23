import { generateSlug } from 'random-word-slugs';
import { prisma } from '../db/prisma';

export function listProjects(userId: string) {
  return prisma.project.findMany({ where: { userId }, orderBy: { createdAt: 'desc' } });
}

export function findProjectByName(name: string) {
  return prisma.project.findFirst({ where: { name } });
}

export function findProjectById(id: string) {
  return prisma.project.findUnique({ where: { id } });
}

export function findProjectBySlug(slug: string) {
  return prisma.project.findUnique({ where: { slug } });
}

export function createProject(userId: string, data: { name: string; gitRepoUrl: string; framework: 'NEXT' | 'REACT' | 'VUE' | 'ANGULAR' | 'SVELTE' | 'OTHER' | 'VITE' }) {
  return prisma.project.create({ data: { ...data, slug: generateSlug(), userId } });
}
