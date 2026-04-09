FROM nginx:alpine
RUN echo 'server { \
    listen 80; \
    server_name localhost; \
    location /health { \
        return 200 "healthy"; \
        add_header Content-Type text/plain; \
    } \
    location / { \
        return 200 "Anomstack service running"; \
        add_header Content-Type text/plain; \
    } \
}' > /etc/nginx/conf.d/default.conf
EXPOSE 80