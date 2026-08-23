FROM node:22-alpine

RUN apk add --no-cache git bash

WORKDIR /app

RUN corepack enable && corepack prepare pnpm@10.12.1 --activate

COPY package.json ./

RUN pnpm install

COPY main.sh ./
COPY src ./src

RUN chmod +x main.sh

ENTRYPOINT ["/bin/bash", "main.sh"]
