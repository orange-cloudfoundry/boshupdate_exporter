package main

import (
	"github.com/orange-cloudfoundry/boshupdate_exporter/boshupdate"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
	"time"
)

var (
	manifestRelease                 *prometheus.GaugeVec
	manifestBoshRelease             *prometheus.GaugeVec
	deploymentStatus                *prometheus.GaugeVec
	deploymentReleaseStatus         *prometheus.GaugeVec
	genericRelease                  *prometheus.GaugeVec
	lastScrapeTimestampMetric       prometheus.Gauge
	lastScrapeErrorMetric           prometheus.Gauge
	lastScrapeDurationSecondsMetric prometheus.Gauge
)

func initMetricsReporter(namespace string, environment string) {
	manifestRelease = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "",
			Name:        "manifest_release",
			Help:        "Seconds from epoch since deployment release is out of date, (0 means up to date)",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
		[]string{"name", "version", "owner", "repo"},
	)

	manifestBoshRelease = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "",
			Name:        "manifest_bosh_release_info",
			Help:        "Informational metric that gives the bosh release versions requests by the latest version of a manifest release, (always 0)",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
		[]string{"manifest_name", "manifest_version", "owner", "repo", "boshrelease_name", "boshrelease_version", "boshrelease_url"},
	)

	genericRelease = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "",
			Name:        "generic_release",
			Help:        "Seconds from epoch since github release is out of date, (0 means up to date)",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
		[]string{"name", "version", "owner", "repo"},
	)

	deploymentStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "",
			Name:        "deployment_status",
			Help:        "Seconds from epoch since this deployment is out of date, (0 means up to date)",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
		[]string{"deployment", "name", "current", "latest"},
	)

	deploymentReleaseStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "",
			Name:        "deployment_bosh_release_status",
			Help:        "Seconds from epoch since this bosh release is out of date, (0 means up to date)",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
		[]string{"deployment", "manifest_name", "manifest_current", "manifest_latest", "boshrelease_name", "boshrelease_current", "boshrelease_latest"},
	)

	lastScrapeTimestampMetric = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "",
			Name:        "last_scrape_timestamp",
			Help:        "Seconds from epoch since last scrape of metrics from boshupdate.",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
	)

	lastScrapeErrorMetric = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "",
			Name:        "last_scrape_error",
			Help:        "Number of errors in last scrape of metrics.",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
	)

	lastScrapeDurationSecondsMetric = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "",
			Name:        "last_scrape_duration",
			Help:        "Duration of the last scrape.",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
	)
}

// getVersion -
// fetch deployment.Versions match manifest.Name
func getVersion(
	deployment boshupdate.BoshDeploymentData,
	releases []boshupdate.ManifestReleaseData) (*boshupdate.ManifestReleaseData, *boshupdate.Version) {

	for _, r := range releases {
		if !r.Match(deployment.ManifestName) {
			continue
		}
		for _, v := range r.Versions {
			if v.Version == deployment.Ref {
				return &r, &v
			}
		}
	}
	return nil, nil
}

func getBoshReleaseVersion(
	manifest *boshupdate.ManifestReleaseData,
	boshRelease boshupdate.BoshRelease) *boshupdate.BoshRelease {
	for _, br := range manifest.BoshReleases {
		if br.Name == boshRelease.Name {
			return &br
		}
	}
	return nil
}

func startUpdate(manager *boshupdate.Manager, interval time.Duration) {
	go func() {
		for {
			log.Debugf("collecting boshupdate metrics")
			startTime := time.Now()
			lastScrapeErrorMetric.Set(0)

			manifests := manager.GetManifestReleases()
			manifestRelease.Reset()
			manifestBoshRelease.Reset()
			for _, m := range manifests {
				if m.HasError {
					log.Warnf("error during analysis of manifest release '%s'", m.Name)
					lastScrapeErrorMetric.Add(1.0)
					continue
				}
				for _, v := range m.Versions {
					manifestRelease.
						WithLabelValues(m.Name, v.Version, m.Owner, m.Repo).
						Set(float64(v.ExpiredSince))
				}
				for _, r := range m.BoshReleases {
					manifestBoshRelease.
						WithLabelValues(m.Name, m.LatestVersion.Version, m.Owner, m.Repo, r.Name, r.Version, r.URL).
						Set(float64(0))
				}
			}

			generics := manager.GetGenericReleases()
			genericRelease.Reset()
			for _, r := range generics {
				if r.HasError {
					log.Warnf("error during analysis of github release '%s'", r.Name)
					lastScrapeErrorMetric.Add(1.0)
					continue
				}
				for _, v := range r.Versions {
					genericRelease.
						WithLabelValues(r.Name, v.Version, r.Owner, r.Repo).
						Set(float64(v.ExpiredSince))
				}
			}

			deployments, err := manager.GetBoshDeployments()
			deploymentStatus.Reset()
			deploymentReleaseStatus.Reset()
			if err != nil {
				log.Errorf("unable to get bosh deployments: %s", err)
				lastScrapeErrorMetric.Add(1.0)
			}

			for _, d := range deployments {
				if d.HasError {
					lastScrapeErrorMetric.Add(1.0)
					log.Warnf("error during analysis of deployment '%s'", d.Deployment)
					continue
				}

				manifest, version := getVersion(d, manifests)
				if manifest == nil || version == nil {
					deploymentStatus.
						WithLabelValues(d.Deployment, d.ManifestName, d.Ref, "not-found").
						Set(0)
				} else {
					deploymentStatus.
						WithLabelValues(d.Deployment, manifest.Name, version.Version, manifest.LatestVersion.Version).
						Set(float64(version.ExpiredSince))
					for _, br := range d.BoshReleases {
						latestBr := getBoshReleaseVersion(manifest, br)
						if latestBr == nil {
							deploymentReleaseStatus.
								WithLabelValues(d.Deployment, manifest.Name, version.Version, manifest.LatestVersion.Version, br.Name, br.Version, "not-found").
								Set(0)
						} else {
							value := version.ExpiredSince
							if br.Version == latestBr.Version {
								value = 0
							}
							deploymentReleaseStatus.
								WithLabelValues(d.Deployment, manifest.Name, version.Version, manifest.LatestVersion.Version, br.Name, br.Version, latestBr.Version).
								Set(float64(value))
						}
					}
				}
			}

			duration := time.Since(startTime).Seconds()
			lastScrapeTimestampMetric.Set(float64(time.Now().Unix()))
			lastScrapeDurationSecondsMetric.Set(duration)
			time.Sleep(interval)
		}
	}()
}

// Local Variables:
// ispell-local-dictionary: "american"
// End:
