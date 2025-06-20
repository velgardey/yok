FROM node:22-alpine

RUN apk add --no-cache git bash

WORKDIR /app

RUN corepack enable && corepack prepare pnpm@10.12.1 --activate

COPY package.json ./

RUN pnpm install

COPY main.sh script.js ./
COPY ca.pem ./

RUN chmod +x main.sh script.js

ENTRYPOINT ["/bin/bash", "main.sh"]