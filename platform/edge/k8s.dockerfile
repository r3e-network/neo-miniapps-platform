# =============================================================================
# Edge Gateway Dockerfile for k3s Deployment
# Deno-based edge runtime with mounted functions
# =============================================================================
FROM denoland/deno:1.45.5

# Set working directory
WORKDIR /app

# Create non-root user for security
RUN groupadd -r deno && useradd -r -g deno deno

# Copy edge functions and dev server
COPY --chown=deno:deno functions/ /app/functions/
COPY --chown=deno:deno dev_server.ts /app/dev_server.ts
COPY --chown=deno:deno deno.json /app/deno.json

# Cache dependencies by running type check
RUN deno cache --reload dev_server.ts

# Switch to non-root user
USER deno

# Expose edge gateway port
EXPOSE 8787

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD deno eval "fetch('http://localhost:8787/health').then(r => r.ok ? Deno.exit(0) : Deno.exit(1))"

# Run dev server with required permissions
CMD ["deno", "run", "--allow-net", "--allow-env", "--allow-read", "dev_server.ts"]
