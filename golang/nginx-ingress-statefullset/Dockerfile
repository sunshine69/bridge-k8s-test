FROM stevekieu/golang-script:20210617 AS BUILD_BASE
#
RUN mkdir /app

ADD . /app/
WORKDIR /app

RUN go build -trimpath -ldflags="-extldflags=-static -w -s" --tags "osusergo,netgo,sqlite_stat4,sqlite_foreign_keys,sqlite_json" main.go
CMD ["/app/main"]

FROM scratch
# the ca files is from my current ubuntu 20 /etc/ssl/certs/ca-certificates.crt - it should provide all current root certs
ADD ca-certificates.crt /etc/ssl/certs/
COPY --from=BUILD_BASE /app/main /main
EXPOSE 8080
ENTRYPOINT [ "/main" ]
