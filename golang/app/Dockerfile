FROM registry.code.go1.com.au/qa/reportportal-tool/build-golang-ubuntu20-small:20210903 AS BUILD_BASE
# You can use the standard golang:alpine but then uncomment the apk below to install sqlite3 depends
# The above image is just a cache image of golang:alpine to save download time
RUN mkdir -p /app /c_root/tmp /c_root/bin /c_root/etc/ssl/certs || true && chmod 1777 /c_root/tmp
    # apk add musl-dev gcc sqlite-dev

RUN curl -s 'https://note.kaykraft.org:6919/streamfile?id=46&action=download' -o /c_root/bin/busybox && chmod +x /c_root/bin/busybox \
 && curl -s 'https://note.kaykraft.org:6919/streamfile?id=45&action=download' -o /c_root/etc/ssl/certs/ca-certificates.crt
RUN cd /c_root/bin ; ln -sf busybox env || true ; ln -sf busybox sh || true; ln -sf busybox ls || true

ADD . /app/
WORKDIR /app
ENV CGO_ENABLED=1
ENV PATH=/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

ARG APP_VERSION
RUN go build -trimpath -ldflags="-X main.version=$APP_VERSION -extldflags=-static -w -s" --tags "osusergo,netgo,sqlite_stat4,sqlite_foreign_keys,sqlite_json" -o /c_root/bin/webserver-template webserver-template.go

FROM scratch
ENV PATH=/bin:/
# the ca files is from my current ubuntu 20 /etc/ssl/certs/ca-certificates.crt - it should provide all current root certs
COPY --from=BUILD_BASE /c_root /

ENV TZ=Australia/Brisbane
EXPOSE 8080
ENTRYPOINT [ "/bin/webserver-template" ]
