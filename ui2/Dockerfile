FROM docker.io/library/node:24-alpine AS runner
WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

RUN addgroup -g 1001 -S nodejs \
  && adduser -S nextjs -G nodejs

COPY .next/standalone ./
COPY .next/static ./.next/static
COPY public ./public 

# -- Create a writable cache directory
RUN mkdir -p .next/cache/images && \
    chown -R nextjs:nodejs .next

USER nextjs
EXPOSE 3000
CMD ["node", "server.js"]
