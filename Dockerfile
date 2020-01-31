FROM alpine
EXPOSE 8080

# Needed by updater to connect to github
RUN apk --update upgrade && \
    apk add curl ca-certificates && \
    update-ca-certificates && \
    rm -rf /var/cache/apk/*

WORKDIR /app
COPY lbrytv_player /app/

CMD ["./lbrytv_player"]
