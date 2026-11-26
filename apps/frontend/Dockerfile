# Stage 1: Build the SvelteKit frontend application
FROM node:22-alpine AS builder

WORKDIR /app

# Copy dependency manifests
COPY package.json package-lock.json .npmrc* ./

# Install dependencies reproducibly using the lockfile
RUN npm ci

# Copy application source files
COPY . .

# Build-time environment variable for API gateway URL
ARG VITE_API_URL=http://localhost:8080
ENV VITE_API_URL=$VITE_API_URL

# Build client and server bundles
RUN npm run build

# Stage 2: Minimal hardened runtime image
FROM node:22-alpine AS runner

WORKDIR /app

# Create unprivileged non-root user and group
RUN addgroup -S -g 10001 appgroup && adduser -S -u 10001 -G appgroup appuser

ENV NODE_ENV=production
ENV HOST=0.0.0.0
ENV PORT=3000

# Copy application artifacts and dependencies from builder stage
COPY --from=builder --chown=appuser:appgroup /app/package.json ./package.json
COPY --from=builder --chown=appuser:appgroup /app/vite.config.ts ./vite.config.ts
COPY --from=builder --chown=appuser:appgroup /app/.svelte-kit ./.svelte-kit
COPY --from=builder --chown=appuser:appgroup /app/static ./static
COPY --from=builder --chown=appuser:appgroup /app/node_modules ./node_modules

# Switch to non-root user
USER appuser

EXPOSE 3000

# Container healthcheck probe
HEALTHCHECK --interval=15s --timeout=5s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:3000/ || exit 1

# Launch the SvelteKit application using the production preview server
CMD ["npx", "vite", "preview", "--host", "0.0.0.0", "--port", "3000"]
