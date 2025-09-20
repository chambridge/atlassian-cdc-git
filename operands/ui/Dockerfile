# Build stage
FROM node:18-alpine AS builder

WORKDIR /app

# Copy package files
COPY operands/ui/package*.json ./
RUN npm ci --only=production

# Copy source code
COPY operands/ui/ ./

# Build the application
RUN npm run build

# Runtime stage
FROM nginx:alpine

# Copy built assets from builder stage
COPY --from=builder /app/build /usr/share/nginx/html

# Copy nginx configuration
COPY operands/ui/nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 3000

CMD ["nginx", "-g", "daemon off;"]