FROM node:22-alpine

RUN apk add --no-cache git
RUN corepack enable && corepack prepare pnpm@10.12.1 --activate

WORKDIR /app

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install

COPY prisma ./prisma/

COPY . .
# Optional Kafka CA: mount ca.pem into the container at runtime and set
# KAFKA_CA_PATH when the broker needs a custom certificate authority.
RUN pnpm prisma generate && pnpm run build

EXPOSE 9000

CMD ["node", "dist/index.js"]
