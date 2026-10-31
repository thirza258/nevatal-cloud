FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata libc6-compat gcompat
WORKDIR /app
COPY bin/nevatal-api /app/nevatal-api
COPY bin/nevatal-migrate /app/nevatal-migrate
COPY migrations /app/migrations

EXPOSE 8090
CMD ["/app/nevatal-api"]
