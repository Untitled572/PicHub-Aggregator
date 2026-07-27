FROM node:20-alpine AS dashboard-builder
WORKDIR /dashboard
COPY dashboard/package*.json ./
RUN npm ci
COPY dashboard/ ./
RUN npm run build

FROM golang:1.26-alpine AS backend-builder
WORKDIR /app
COPY backend/go.* ./
RUN go mod download
COPY backend/ ./
COPY --from=dashboard-builder /dashboard/dist ./embed/dist
RUN CGO_ENABLED=0 go build -o pichub .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend-builder /app/pichub .
COPY backend/config.json .
EXPOSE 5721
CMD ["./pichub"]
