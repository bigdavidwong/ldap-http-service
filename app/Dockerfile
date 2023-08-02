FROM ubuntu
RUN apt-get update && apt-get install -y curl
RUN mkdir /app
COPY main /app/main
EXPOSE $GIN_PORT
WORKDIR /app
ENTRYPOINT ["/app/main"]