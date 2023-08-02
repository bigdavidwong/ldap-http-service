FROM ubuntu
RUN apt-get update && apt-get install -y curl
RUN mkdir /app
COPY bin/amd64/app /app/main
EXPOSE $GIN_PORT
WORKDIR /app
ENTRYPOINT ["/app/main"]