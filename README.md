# Boshupdate Prometheus Exporter [![Build Status](https://travis-ci.org/orange-cloudfoundry/boshupdate_exporter.png)](https://travis-ci.org/orange-cloudfoundry/boshupdate_exporter)

A [Prometheus][prometheus] exporter that identifies out of date [BOSH][bosh] deployments.

It queries [Github][github] and fetches available releases of canonical [BOSH][bosh] manifests
such as [cf-deployment][cf-deployment], then conciliates with actual running deployments
fetched from [BOSH][bosh]  director.

It is also capable of analyzing [BOSH][bosh] manifests in order to extract recommended
versions of [BOSH][bosh] releases.

## Installation

### Binaries

Download the already existing [binaries][binaries] for your platform:

```bash
$ ./boshupdate_exporter <flags>
```

### Docker

To run the boshupdate exporter as a Docker container, run:

```bash
$ docker run -p 9362:9362 orangeopensource/boshupdate-exporter <flags>
```

### BOSH

This exporter can be deployed using the [Githubexporter BOSH Release][githubexporter-boshrelease].

## Usage

### Github Token

In order to connect to the [Github API][github_api] a `token` must be provided.
The `token` can be created by following the [Github HowTo][github-create-token]

### Bosh deployment prerequites

The exporter identifies the version of a running deployment by extracting the `manifest_version`.

This key is already built-in some canonical manifests like [cf-deployment][cf-deployment]
but must be manually added in others using the operator `set-manifest-version.yml`

```yml
- type: replace
  path: /manifest_version?
  value: v((version))
```

### Exporter Configuration

The provided [sample configuration](config.yml.sample) is a good starting point.


#### Detailed Specification

* General structure

```yaml
bosh:
  log_level: <log_level>
  url:       <url>        # url (scheme://host:port) to director endpoint
  ca_cert:   <path>       # path to director CA certificate
  client_id: <string>     # client id
  client_secret: <string> # client secret
  proxy: <url>            # proxy url, if any.
  excludes: list[regexp]  # list of bosh deployment to exclude from scrap

github:
  token: <string>                          # your github token here
  update_interval: 4h                      # interval between two github updates
  manifest_releases: map[string, manifest] # list of canonical manifests to monitor
  generic_releases:  map[string, generic]  # list of generic github release to monitor
```


* *manifest*

```yaml
<name>:
    types: *release-types*
    format: *release-formatter*
    owner: <string>        # github project's owner or organization
    repo: <string>         # github project's name
    manifest: <string>     # remote path to main BOSH manifest
    ops: list[string]      # list of remote ops-file paths to apply to main manifest
    vars: list[string]     # list of remote vars-file paths to apply to main manifest
    matchers: list[string] # list of regexp that match running deployments names
```

* *release*

```yaml
<name>:
    types: *release-types*
    format: *release-formatter*
    owner: <string>     # github project's owner or organization
    repo: <string>      # github project's name
```

* *release-types*

```
# List of objects types to consider as a release.
list[string]

# String must be one or more of the following values:
# - release:       Github release which is neither in 'draft' nor 'pre' state
# - pre_release:   Github release in 'pre' state
# - draft_release: Github release in 'draft' state
# - tag:           Github tag
```

* *format*

```yaml
# Format tells how to parse detected release name into a version
format:
  match: <regexp>   # a regex to match release name
  replace: <string> # a replacement for matched release name

# When not provided, the default format value is
# format:
#   match: "v([0-9.]+)"
#  replace: "${1}"
```

### Flags

| Flag / Environment Variable                                          | Required | Default         | Description                                                                                                                                                                                                                           |
| ---------------------------                                          | -------- | -------         | -----------                                                                                                                                                                                                                           |
| `config`<br />`BOSHUPDATE_EXPORTER_CONFIG`                           | No       | `config.yml`    | Path to configuration file                                                                                                                                                                                                            |
| `metrics.namespace`<br />`BOSHUPDATE_EXPORTER_METRICS_NAMESPACE`     | No       | `boshupdate`    | Metrics Namespace                                                                                                                                                                                                                     |
| `metrics.environment`<br />`BOSHUPDATE_EXPORTER_METRICS_ENVIRONMENT` | Yes      |                 | `environment` label to be attached to metrics                                                                                                                                                                                         |
| `web.listen-address`<br />`BOSHUPDATE_EXPORTER_WEB_LISTEN_ADDRESS`   | No       | `:9362`         | Address to listen on for web interface and telemetry                                                                                                                                                                                  |
| `web.telemetry-path`<br />`BOSHUPDATE_EXPORTER_WEB_TELEMETRY_PATH`   | No       | `/metrics`      | Path under which to expose Prometheus metrics                                                                                                                                                                                         |
| `web.auth.username`<br />`BOSHUPDATE_EXPORTER_WEB_AUTH_USERNAME`     | No       |                 | Username for web interface basic auth                                                                                                                                                                                                 |
| `web.auth.password`<br />`BOSHUPDATE_EXPORTER_WEB_AUTH_PASSWORD`     | No       |                 | Password for web interface basic auth                                                                                                                                                                                                 |
| `web.tls.cert_file`<br />`BOSHUPDATE_EXPORTER_WEB_TLS_CERTFILE`      | No       |                 | Path to a file that contains the TLS certificate (PEM format). If the certificate is signed by a certificate authority, the file should be the concatenation of the server's certificate, any intermediates, and the CA's certificate |
| `web.tls.key_file`<br />`BOSHUPDATE_EXPORTER_WEB_TLS_KEYFILE`        | No       |                 | Path to a file that contains the TLS private key (PEM format)                                                                                                                                                                         |


### Metrics

The exporter returns the following  metrics:


| Metric                                         | Description                                                                                   | Labels                                                                                     |
|------------------------------------------------|-----------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------|
| *metrics.namespace*_manifest_release           | Seconds from epoch since canonical manifest version if out-of-date, 0 means up-to-date        | `environment`, `name`, `version`, `owner`, `repo`                                          |
| *metrics.namespace*_manifest_bosh_release_info | Information about recommended bosh releases used by last available canonical manifest release | `environment`, `manifest_name`, `onwer`, `repo`, `boshrelease_name`, `boshrelease_version` |
| *metrics.namespace*_generic_release            | Seconds from epoch since repository version is out-of-date, 0 means up-to-date                | `environment`, `name`, `version`, `owner`, `repo`                                          |
| *metrics.namespace*_deployment_status          | Seconds from epoch since deployment is out-of-date, , 0 means up-to-date                      | `environment`, `name`, `current`, `latest`                                                 |
| *metrics.namespace*_last_scrape_timestamp      | Seconds from epoch since last scrape of metrics from boshupdate                               | `environment`                                                                              |
| *metrics.namespace*_last_scrape_error          | Number of errors in last scrape of metrics                                                    | `environment`                                                                              |
| *metrics.namespace*_last_scrape_duration       | Duration of the last scrape                                                                   | `environment`                                                                              |

## Contributing

Refer to the [contributing guidelines][contributing].

## License

Apache License 2.0, see [LICENSE][license].

[binaries]: https://github.com/orange-cloudfoundry/boshupdate_exporter/releases
[github]: https://github.com/cloudfoundry-incubator/github
[github_api]: https://developer.github.com/v3/
[contributing]: https://github.com/orange-cloudfoundry/boshupdate_exporter/blob/master/CONTRIBUTING.md
[golang]: https://golang.org/
[license]: https://github.com/orange-cloudfoundry/boshupdate_exporter/blob/master/LICENSE
[prometheus]: https://prometheus.io/
[github-create-token]: https://help.github.com/articles/creating-a-personal-access-token-for-the-command-line/
[bosh]: https://bosh.io
[cf-deployment]: https://github.com/cloudfoundry/cf-deployment
<!-- Local Variables: -->
<!-- ispell-local-dictionary: "american" -->
<!-- End: -->
