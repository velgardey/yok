FROM node:22-alpine

RUN apk add --no-cache git
RUN corepack enable && corepack prepare pnpm@10.12.1 --activate

WORKDIR /app

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install

COPY prisma ./prisma/
COPY ca.pem ./

COPY . .
RUN pnpm prisma generate
RUN pnpm run build

EXPOSE 9000

CMD ["node", "index.js"]