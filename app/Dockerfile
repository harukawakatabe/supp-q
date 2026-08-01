FROM node:24-alpine AS build

WORKDIR /src
RUN npm install --global pnpm@11.9.0
COPY app/package.json app/pnpm-lock.yaml app/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY app/ .
RUN pnpm type-check && pnpm test && pnpm build:h5

FROM caddy:2.11.4-alpine
COPY --from=build /src/dist/build/h5 /srv
COPY deploy/Caddyfile.local /etc/caddy/Caddyfile
EXPOSE 80
