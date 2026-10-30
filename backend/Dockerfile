FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY bin/nevatal-api /app/nevatal-api

EXPOSE 8090
CMD ["/app/nevatal-api"]
