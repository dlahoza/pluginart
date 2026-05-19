FROM node:22-alpine AS runtime
WORKDIR /runtime
COPY runtimes/typescript/package*.json ./
COPY runtimes/typescript/tsconfig.json ./
COPY runtimes/typescript/src ./src
RUN npm ci && npm run build

FROM node:22-alpine AS builder
WORKDIR /src
COPY --from=runtime /runtime /runtime
COPY examples/plugin-repeat-ts/package.json ./package.json
RUN npm pkg set dependencies.pluginart=file:/runtime && npm install
COPY examples/plugin-repeat-ts/tsconfig.json ./tsconfig.json
COPY examples/plugin-repeat-ts/*.ts ./
COPY examples/plugin-repeat-ts/plugin ./plugin
RUN npm run build

FROM node:22-alpine
WORKDIR /app
COPY --from=runtime /runtime /runtime
COPY --from=builder /src/dist ./dist
COPY --from=builder /src/node_modules ./node_modules
ENTRYPOINT ["node", "dist/plugin.js"]
