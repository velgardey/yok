import cors from 'cors';
import express from 'express';
import type { AppConfig } from './config';
import { LogStore } from './logs/clickhouse';
import { authRouter } from './routes/auth';
import { deploymentsRouter } from './routes/deployments';
import { healthRouter } from './routes/health';
import { projectsRouter } from './routes/projects';
import { resolveRouter } from './routes/resolve';

export function createApp(config: AppConfig, logStore: LogStore): express.Express {
  const app = express();
  app.use(cors());
  app.use(express.json());

  app.use(healthRouter);
  app.use(authRouter(config));
  app.use(projectsRouter);
  app.use(deploymentsRouter(config, logStore));
  app.use(resolveRouter(config.proxyServiceToken));

  app.use((err: unknown, _req: express.Request, res: express.Response, _next: express.NextFunction) => {
    console.error('Unhandled error:', err);
    res.status(500).json({ status: 'error', message: 'Internal server error' });
  });

  return app;
}
