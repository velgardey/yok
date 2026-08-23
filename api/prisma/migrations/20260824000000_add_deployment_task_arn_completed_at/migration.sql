-- AlterTable
ALTER TABLE "Deployment" ADD COLUMN "task_arn" TEXT;
ALTER TABLE "Deployment" ADD COLUMN "completed_at" TIMESTAMP(3);
