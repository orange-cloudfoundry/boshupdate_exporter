# GithubRelease Prometheus Exporter [![Build Status](https://travis-ci.org/orange-cloudfoundry/githubrelease_exporter.png)](https://travis-ci.org/orange-cloudfoundry/githubrelease_exporter)

A [Prometheus][prometheus] exporter for [Github][github]. The exporter exports informational metrics
about available releases on github. It is also capable of analyzing [BOSH][bosh] deployment manifests
in order to extract recommended versions of [BOSH][bosh] releases.

## Installation

### Binaries

Download the already existing [binaries][binaries] for your platform:

```bash
$ ./githubrelease_exporter <flags>
```

### Docker

To run the githubrelease exporter as a Docker container, run:

```bash
$ docker run -p 9362:9362 orangeopensource/githubrelease-exporter <flags>
```

### BOSH

This exporter can be deployed using the [Githubexporter BOSH Release][githubexporter-boshrelease].

## Usage

### Github Token

In order to connect to the [Github API][github_api] a `token` must be provided.
The `token` can be created by following the [Github HowTo][github-create-token]

### Configuration

The provided [sample configuration](config.yml.sample) is a good starting point.


#### Detailed Specification

* General structure

```yaml
github-token: <string> # valid token for github api
bosh-deployment: map[string, *deployment*]
github-release:  map[string, *release*]
```


* *deployment*

```yaml
<name>:
    types: *release-types*
    format: *release-formatter*
    owner: <string>     # github project's owner or organization
    repo: <string>      # github project's name
    manifest: <string>  # remote path to main BOSH manifest
    ops: list[string]   # list of remote ops-file paths to apply to main manifest
    vars: list[string]  # list of remote vars-file paths to apply to main manifest
```

* *release*

```yaml
<name>:
    types: *release-types*
    format: *release-formatter*
    owner: <string>     # github project's owner or organization
    repo: <string>      # github project's name
```

* *types*

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

| Flag / Environment Variable                                             | Required | Default         | Description                                                                                                                                                                                                                           |
| ---------------------------                                             | -------- | -------         | -----------                                                                                                                                                                                                                           |
| `config`<br />`GITHUBRELEASE_EXPORTER_CONFIG`                           | No       | `config.yml`    | Path to configuration file                                                                                                                                                                                                            |
| `metrics.namespace`<br />`GITHUBRELEASE_EXPORTER_METRICS_NAMESPACE`     | No       | `githubrelease` | Metrics Namespace                                                                                                                                                                                                                     |
| `metrics.environment`<br />`GITHUBRELEASE_EXPORTER_METRICS_ENVIRONMENT` | Yes      |                 | `environment` label to be attached to metrics                                                                                                                                                                                         |
| `web.listen-address`<br />`GITHUBRELEASE_EXPORTER_WEB_LISTEN_ADDRESS`   | No       | `:9362`         | Address to listen on for web interface and telemetry                                                                                                                                                                                  |
| `web.telemetry-path`<br />`GITHUBRELEASE_EXPORTER_WEB_TELEMETRY_PATH`   | No       | `/metrics`      | Path under which to expose Prometheus metrics                                                                                                                                                                                         |
| `web.auth.username`<br />`GITHUBRELEASE_EXPORTER_WEB_AUTH_USERNAME`     | No       |                 | Username for web interface basic auth                                                                                                                                                                                                 |
| `web.auth.password`<br />`GITHUBRELEASE_EXPORTER_WEB_AUTH_PASSWORD`     | No       |                 | Password for web interface basic auth                                                                                                                                                                                                 |
| `web.tls.cert_file`<br />`GITHUBRELEASE_EXPORTER_WEB_TLS_CERTFILE`      | No       |                 | Path to a file that contains the TLS certificate (PEM format). If the certificate is signed by a certificate authority, the file should be the concatenation of the server's certificate, any intermediates, and the CA's certificate |
| `web.tls.key_file`<br />`GITHUBRELEASE_EXPORTER_WEB_TLS_KEYFILE`        | No       |                 | Path to a file that contains the TLS private key (PEM format)                                                                                                                                                                         |


### Metrics

The exporter returns the following  metrics:

| Metric                                    | Description                                                                       | Labels                                                                                                           |
| ------                                    | -----------                                                                       | ------                                                                                                           |
| *metrics.namespace*_bosh_release_info     | Informations about recommended bosh releases used by a particular bosh deployment | `environment`, `deployment_name`, `github_repo`, `manifest_version`, `bosh_release_name`, `bosh_release_version` |
| *metrics.namespace*_release_info          | Informations about available github releases a given repository                   | `environment`, `release_name`, `github_repo`, `release_version`                                                  |
| *metrics.namespace*_last_scrape_timestamp | Number of seconds since 1970 since last scrape of metrics                         | `environment`                                                                                                    |

## Contributing

Refer to the [contributing guidelines][contributing].

## License

Apache License 2.0, see [LICENSE][license].

[binaries]: https://github.com/orange-cloudfoundry/githubrelease_exporter/releases
[github]: https://github.com/cloudfoundry-incubator/github
[github_api]: https://developer.github.com/v3/
[contributing]: https://github.com/orange-cloudfoundry/githubrelease_exporter/blob/master/CONTRIBUTING.md
[golang]: https://golang.org/
[license]: https://github.com/orange-cloudfoundry/githubrelease_exporter/blob/master/LICENSE
[prometheus]: https://prometheus.io/
[githubrelease-boshrelease]: https://github.com/bosh-prometheus/prometheus-boshrelease
[github-create-token]: https://help.github.com/articles/creating-a-personal-access-token-for-the-command-line/

<!-- Local Variables: -->
<!-- ispell-local-dictionary: "en" -->
<!-- End: -->
