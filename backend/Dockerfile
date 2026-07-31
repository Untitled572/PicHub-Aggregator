FROM node:20-alpine AS dashboard-builder
ARG VITE_APP_VERSION=dev
ENV VITE_APP_VERSION=$VITE_APP_VERSION
WORKDIR /dashboard
COPY dashboard/package*.json ./
RUN npm ci
COPY dashboard/ ./
RUN npx vue-tsc --noEmit && npx vite build

FROM golang:1.26-alpine AS backend-builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY backend/go.* ./
RUN go mod download
COPY backend/ ./
COPY --from=dashboard-builder /dashboard/dist ./embed/dist
RUN CGO_ENABLED=1 go build -o pichub .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata su-exec
WORKDIR /app
COPY --from=backend-builder /app/pichub .
COPY backend/config/config.json .
COPY backend/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh
EXPOSE 5721
ENTRYPOINT ["/app/entrypoint.sh"]
