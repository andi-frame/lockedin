# Nightly pg_dump into the Garage bucket. pg_dump must be at least the server's major version,
# so this starts from the same Postgres image as compose.yaml; aws-cli does the S3 side.
FROM postgres:18-alpine
RUN apk add --no-cache aws-cli tzdata
COPY backup.sh restore.sh /usr/local/bin/
ENV HOME=/tmp
USER 10001:10001
ENTRYPOINT ["sh", "/usr/local/bin/backup.sh"]
