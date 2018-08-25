FROM        quay.io/prometheus/busybox:latest
MAINTAINER  Xavier MARCELET <xavier.marcelet@orange.com>

COPY githubrelease_exporter /bin/githubrelease_exporter

ENTRYPOINT ["/bin/githubrelease_exporter"]
EXPOSE     9362
