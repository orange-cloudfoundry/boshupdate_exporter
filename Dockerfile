FROM        quay.io/prometheus/busybox:latest
MAINTAINER  Xavier MARCELET <xavier.marcelet@orange.com>

COPY boshupdate_exporter /bin/boshupdate_exporter

ENTRYPOINT ["/bin/boshupdate_exporter"]
EXPOSE     9362
