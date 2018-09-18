package main

import (
	"fmt"
	"github.com/orange-cloudfoundry/githubrelease_exporter/githubrelease"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"time"
)

// GithubCollector -
type GithubCollector struct {
	manager                         githubrelease.Manager
	deploymentMetrics               *prometheus.GaugeVec
	boshDeploymentMetrics           *prometheus.GaugeVec
	githubReleaseMetrics            *prometheus.GaugeVec
	lastScrapeTimestampMetric       prometheus.Gauge
	lastScrapeErrorMetric           prometheus.Gauge
	lastScrapeDurationSecondsMetric prometheus.Gauge
}

// NewGithubCollector -
func NewGithubCollector(environment string, manager githubrelease.Manager) *GithubCollector {
	deploymentMetrics = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "githubrelease",
			Subsystem:   "",
			Name:        "deployment_info",
			Help:        "Wheter last scrap bosh manifests versions resulted in an error (1 for error, 0 for success)",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
		[]string{"deployment_name", "manifest_name", "version"},
	)

	boshDeploymentMetrics = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "githubrelease",
			Subsystem:   "",
			Name:        "bosh_release_info",
			Help:        "Wheter last scrap interpolation of bosh manifests resulted in an error (1 for error, 0 for success)",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
		[]string{"deployment_name", "github_repo", "manifest_version", "bosh_release_name", "bosh_release_version"},
	)

	githubReleaseMetrics = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "githubrelease",
			Subsystem:   "",
			Name:        "release_info",
			Help:        "Wheter last scrap of github release resulted in an error (1 for error, 0 for success)",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
		[]string{"release_name", "github_repo", "release_version"},
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
		deploymentMetrics:               deploymentMetrics,
		boshDeploymentMetrics:           boshDeploymentMetrics,
		githubReleaseMetrics:            githubReleaseMetrics,
		lastScrapeTimestampMetric:       lastScrapeTimestampMetric,
		lastScrapeErrorMetric:           lastScrapeErrorMetric,
		lastScrapeDurationSecondsMetric: lastScrapeDurationSecondsMetric,
	}
}

func (c GithubCollector) Collect(ch chan<- prometheus.Metric) {
	log.Debugf("collecting githubrelease metrics")

	startTime := time.Now()
	c.lastScrapeErrorMetric.Set(0.0)
	c.lastScrapeTimestampMetric.Set(float64(time.Now().Unix()))

	deployments := c.manager.GetBoshDeployments()
	for _, d := range deployments {
		if d.HasError {
			c.lastScrapeErrorMetric.Add(1.0)
		}

		for _, r := range d.Releases {
			c.boshDeploymentMetrics.
				WithLabelValues(d.Deployment, fmt.Sprintf("%s/%s", d.Owner, d.Repo), d.ReleaseData.Version, r.Name, r.Version).
				Set(float64(d.ReleaseData.Time))
		}
	}

	manifests, err := c.manager.GetManifests()
	if err != nil {
		c.lastScrapeErrorMetric.Add(1.0)
	}
	for _, manifest := range manifests {
		if manifest.HasError {
			c.lastScrapeErrorMetric.Add(1.0)
		}
		c.deploymentMetrics.
			WithLabelValues(manifest.Deployment, manifest.Name, manifest.Version).
			Set(1)
	}

	releases := c.manager.GetGithubReleases()
	for _, r := range releases {
		if r.HasError {
			c.lastScrapeErrorMetric.Add(1.0)
		}
		c.githubReleaseMetrics.
			WithLabelValues(r.Name, fmt.Sprintf("%s/%s", r.Owner, r.Repo), r.Version).
			Set(float64(r.ReleaseData.Time))
	}

	duration := time.Now().Sub(startTime).Seconds()
	c.lastScrapeDurationSecondsMetric.Set(duration)

	c.deploymentMetrics.Collect(ch)
	c.boshDeploymentMetrics.Collect(ch)
	c.githubReleaseMetrics.Collect(ch)
	c.lastScrapeTimestampMetric.Collect(ch)
	c.lastScrapeErrorMetric.Collect(ch)
	c.lastScrapeDurationSecondsMetric.Collect(ch)
}

func (c GithubCollector) Describe(ch chan<- *prometheus.Desc) {
	c.deploymentMetrics.Describe(ch)
	c.boshDeploymentMetrics.Describe(ch)
	c.githubReleaseMetrics.Describe(ch)
	c.lastScrapeTimestampMetric.Describe(ch)
	c.lastScrapeErrorMetric.Describe(ch)
	c.lastScrapeDurationSecondsMetric.Describe(ch)
}
