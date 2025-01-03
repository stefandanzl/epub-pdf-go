FROM golang:alpine

ENV WEBDAV_URL=http://localhost \
    WEBDAV_USERNAME=user \
    WEBDAV_PASSWORD=pass \
    QT_QPA_PLATFORM=offscreen \
    QTWEBENGINE_DISABLE_SANDBOX=1 \
    QT_QUICK_BACKEND=software \
    XDG_RUNTIME_DIR=/tmp/runtime-appuser

RUN echo "http://dl-cdn.alpinelinux.org/alpine/edge/testing" >> /etc/apk/repositories && \
    echo "http://dl-cdn.alpinelinux.org/alpine/edge/community" >> /etc/apk/repositories && \
    apk update && \
    apk add --no-cache calibre

RUN adduser -D appuser && \
    mkdir -p /app/temp $XDG_RUNTIME_DIR && \
    chmod 700 $XDG_RUNTIME_DIR && \
    chown -R appuser:appuser /app $XDG_RUNTIME_DIR

WORKDIR /app

COPY --chown=appuser:appuser . .

USER appuser

RUN go build -o main .

EXPOSE 3000

CMD ["./main"]
