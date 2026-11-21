# Production image of the Next.js frontend (standalone server). Built in GitHub Actions.
# NEXT_PUBLIC_API_URL is inlined into the browser bundle at build time: the image is tied to one site URL.
FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund

FROM node:22-alpine AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
ARG NEXT_PUBLIC_API_URL=http://localhost:8448
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL NEXT_TELEMETRY_DISABLED=1
RUN npm run build

FROM node:22-alpine
ARG VERSION=dev
LABEL org.opencontainers.image.source="https://github.com/almukhanbetov/ortalyq-new" \
      org.opencontainers.image.title="ortalyq-new-web" \
      org.opencontainers.image.revision="$VERSION"
WORKDIR /app
ENV NODE_ENV=production PORT=3448 HOSTNAME=0.0.0.0 NEXT_TELEMETRY_DISABLED=1
RUN adduser -D -u 10001 app
COPY --from=build --chown=app /app/.next/standalone ./
COPY --from=build --chown=app /app/.next/static ./.next/static
COPY --from=build --chown=app /app/public ./public
USER app
EXPOSE 3448
CMD ["node", "server.js"]
