# NEXT_PUBLIC_* values are inlined into the browser bundle at build time, so pass the
# public URLs of YOUR deployment as build arguments:
#   docker build --build-arg NEXT_PUBLIC_API_URL=https://api.example.com \
#                --build-arg NEXT_PUBLIC_INGEST_URL=https://ingest.example.com -t web .
FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
ARG NEXT_PUBLIC_API_URL=http://localhost:8000
ARG NEXT_PUBLIC_INGEST_URL=http://localhost:4318
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL NEXT_PUBLIC_INGEST_URL=$NEXT_PUBLIC_INGEST_URL NEXT_TELEMETRY_DISABLED=1
RUN npm run build

FROM node:22-alpine
WORKDIR /app
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PORT=3000 HOSTNAME=0.0.0.0
COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static
COPY --from=build /app/public ./public
USER node
EXPOSE 3000
CMD ["node", "server.js"]
