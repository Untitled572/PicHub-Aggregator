#!/bin/sh
PUID=${PUID:-1000}
PGID=${PGID:-1000}

addgroup -g "$PGID" pichub
adduser -D -u "$PUID" -G pichub pichub

mkdir -p /app/data /app/cache
chown -R "$PUID:$PGID" /app/data /app/cache

exec su-exec "$PUID:$PGID" ./pichub