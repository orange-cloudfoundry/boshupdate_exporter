package main

import (
	"github.com/orange-cloudfoundry/githubrelease_exporter/githubrelease"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"time"
)

// GithubCollector -
type GithubCollector struct {
	manager                         githubrelease.Manager
	manifestRelease                 *prometheus.GaugeVec
	manifestBoshRelease             *prometheus.GaugeVec
	deploymentStatus                *prometheus.GaugeVec
	genericRelease                  *prometheus.GaugeVec
	lastScrapeTimestampMetric       prometheus.Gauge
	lastScrapeErrorMetric           prometheus.Gauge
	lastScrapeDurationSecondsMetric prometheus.Gauge
}

// NewGithubCollector -
func NewGithubCollector(environment string, manager githubrelease.Manager) *GithubCollector {
	manifestRelease := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "githubrelease",
			Subsystem:   "",
			Name:        "manifest_release",
			Help:        "Seconds from epoch since deployment release is out of date, (0 means up to date)",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
		[]string{"name", "version", "owner", "repo"},
	)

	manifestBoshRelease := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "githubrelease",
			Subsystem:   "",
			Name:        "manifest_bosh_release_info",
			Help:        "Informational metric that gives the bosh release versions requests by the lastest version of a manifest release, (always 0)",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
		[]string{"manifest_name", "manifest_version", "owner", "repo", "boshrelease_name", "boshrelease_version"},
	)

	genericRelease := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "githubrelease",
			Subsystem:   "",
			Name:        "generic_release",
			Help:        "Seconds from epoch since this github release is out of date, (0 means up to date)",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
		[]string{"name", "version", "owner", "repo"},
	)

	deploymentStatus := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "githubrelease",
			Subsystem:   "",
			Name:        "deployment_status",
			Help:        "Seconds from epoch since this deployment is out of date, (0 means up to date)",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
		[]string{"deployment", "name", "current", "latest"},
	)

	lastScrapeTimestampMetric := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   "githubrelease",
			Subsystem:   "",
			Name:        "last_scrape_timestamp",
			Help:        "Number of seconds since 1970 since last scrape of metrics from githubrelease.",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
	)

	lastScrapeErrorMetric := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   "githubrelease",
			Subsystem:   "",
			Name:        "last_scrape_error",
			Help:        "Whether the last scrape of metrics resulted in an error (1 for error, 0 for success).",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
	)

	lastScrapeDurationSecondsMetric := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   "githubrelease",
			Subsystem:   "",
			Name:        "last_scrape_duration",
			Help:        "Duration of the last scrape.",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
	)

	return &GithubCollector{
		manager:                         manager,
		manifestRelease:                 manifestRelease,
		manifestBoshRelease:             manifestBoshRelease,
		deploymentStatus:                deploymentStatus,
		genericRelease:                  genericRelease,
		lastScrapeTimestampMetric:       lastScrapeTimestampMetric,
		lastScrapeErrorMetric:           lastScrapeErrorMetric,
		lastScrapeDurationSecondsMetric: lastScrapeDurationSecondsMetric,
	}
}

// getVersion -
// fetch deployment.Versions match manifest.Name
func (c GithubCollector) getVersion(
	deployment githubrelease.BoshDeploymentData,
	releases []githubrelease.ManifestReleaseData) (*githubrelease.ManifestReleaseData, *githubrelease.Version) {

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

// Collect -
func (c GithubCollector) Collect(ch chan<- prometheus.Metric) {
	log.Debugf("collecting githubrelease metrics")

	startTime := time.Now()
	c.lastScrapeErrorMetric.Set(0.0)
	c.lastScrapeTimestampMetric.Set(float64(time.Now().Unix()))

	manifests := c.manager.GetManifestReleases()
	for _, m := range manifests {
		if m.HasError {
			log.Warnf("error during analysis of manifest release '%s'", m.Name)
			c.lastScrapeErrorMetric.Add(1.0)
			continue
		}
		for _, v := range m.Versions {
			c.manifestRelease.
				WithLabelValues(m.Name, v.Version, m.Owner, m.Repo).
				Set(float64(v.ExpiredSince))
		}

		for _, r := range m.BoshReleases {
			c.manifestBoshRelease.
				WithLabelValues(m.Name, m.LatestVersion.Version, m.Owner, m.Repo, r.Name, r.Version).
				Set(float64(0))
		}
	}

	generics := c.manager.GetGenericReleases()
	for _, r := range generics {
		if r.HasError {
			log.Warnf("error during analysis of github release '%s'", r.Name)
			c.lastScrapeErrorMetric.Add(1.0)
			continue
		}
		for _, v := range r.Versions {
			c.genericRelease.
				WithLabelValues(r.Name, v.Version, r.Owner, r.Repo).
				Set(float64(v.ExpiredSince))
		}
	}

	deployments, err := c.manager.GetBoshDeployments()
	if err != nil {
		log.Errorf("unable to get bosh deployments: %s", err)
		c.lastScrapeErrorMetric.Add(1.0)
	}

	for _, d := range deployments {
		if d.HasError {
			c.lastScrapeErrorMetric.Add(1.0)
			log.Warnf("error during analysis of deployment '%s'", d.Deployment)
			continue
		}

		manifest, version := c.getVersion(d, manifests)
		if manifest == nil || version == nil {
			c.deploymentStatus.
				WithLabelValues(d.Deployment, d.ManifestName, d.Ref, "not-found").
				Set(0)
		} else {
			c.deploymentStatus.
				WithLabelValues(d.Deployment, manifest.Name, version.Version, manifest.LatestVersion.Version).
				Set(float64(version.ExpiredSince))
		}
	}

	duration := time.Since(startTime).Seconds()

	c.lastScrapeDurationSecondsMetric.Set(duration)
	c.manifestRelease.Collect(ch)
	c.manifestBoshRelease.Collect(ch)
	c.deploymentStatus.Collect(ch)
	c.genericRelease.Collect(ch)
	c.lastScrapeTimestampMetric.Collect(ch)
	c.lastScrapeErrorMetric.Collect(ch)
	c.lastScrapeDurationSecondsMetric.Collect(ch)
}

// Describe -
func (c GithubCollector) Describe(ch chan<- *prometheus.Desc) {
	c.manifestRelease.Describe(ch)
	c.manifestBoshRelease.Describe(ch)
	c.deploymentStatus.Describe(ch)
	c.genericRelease.Describe(ch)
	c.lastScrapeTimestampMetric.Describe(ch)
	c.lastScrapeErrorMetric.Describe(ch)
	c.lastScrapeDurationSecondsMetric.Describe(ch)
}

// Local Variables:
// ispell-local-dictionary: "american"
// End:
