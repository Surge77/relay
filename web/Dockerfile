# Next.js standalone build. NEXT_PUBLIC_GATEWAY_WS is inlined at build time, so
# it must be supplied as a build arg (the browser reads it to reach the LB).
FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

FROM node:22-alpine AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
ARG NEXT_PUBLIC_GATEWAY_WS=ws://localhost:8080/ws
ENV NEXT_PUBLIC_GATEWAY_WS=${NEXT_PUBLIC_GATEWAY_WS}
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

FROM node:22-alpine AS run
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV PORT=3000
# Standalone output bundles only the server and its runtime deps.
COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static
USER node
EXPOSE 3000
CMD ["node", "server.js"]
