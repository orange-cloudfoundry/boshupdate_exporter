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
	manager                   githubrelease.Manager
	boshDeploymentMetrics     *prometheus.GaugeVec
	githubReleaseMetrics      *prometheus.GaugeVec
	lastScrapeTimestampMetric prometheus.Gauge
}

// NewGithubCollector -
func NewGithubCollector(environment string, manager githubrelease.Manager) *GithubCollector {

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

	lastScrapeTimesptampMetric := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   "githubrelease",
			Subsystem:   "",
			Name:        "last_scrape_timestamp",
			Help:        "Number of seconds since 1970 since last scrape of metrics from githubrelease.",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
	)

	return &GithubCollector{
		manager:                   manager,
		boshDeploymentMetrics:     boshDeploymentMetrics,
		githubReleaseMetrics:      githubReleaseMetrics,
		lastScrapeTimestampMetric: lastScrapeTimesptampMetric,
	}
}

func (c GithubCollector) Collect(ch chan<- prometheus.Metric) {
	log.Debugf("collecting githubrelease metrics")
	c.lastScrapeTimestampMetric.Set(float64(time.Now().Unix()))

	deployments := c.manager.GetBoshDeployments()
	for _, d := range deployments {
		value := 0
		if d.HasError {
			value = 1
		}
		for _, r := range d.Releases {
			c.boshDeploymentMetrics.
				WithLabelValues(d.Deployment, fmt.Sprintf("%s/%s", d.Owner, d.Repo), d.ReleaseData.Version, r.Name, r.Version).
				Set(float64(value))
		}
		// if 0 == len(d.Releases) {
		// 	c.boshDeploymentMetrics.
		// 		WithLabelValues(d.Deployment, fmt.Sprintf("%s/%s", d.Owner, d.Repo), d.ReleaseData.Version, "", "").
		// 		Set(float64(1))
		// }
	}

	releases := c.manager.GetGithubReleases()
	for _, r := range releases {
		value := 0
		if r.HasError {
			value = 1
		}
		c.githubReleaseMetrics.
			WithLabelValues(r.Name, fmt.Sprintf("%s/%s", r.Owner, r.Repo), r.Version).
			Set(float64(value))
	}

	c.boshDeploymentMetrics.Collect(ch)
	c.githubReleaseMetrics.Collect(ch)
	c.lastScrapeTimestampMetric.Collect(ch)
}

func (c GithubCollector) Describe(ch chan<- *prometheus.Desc) {
	c.boshDeploymentMetrics.Describe(ch)
	c.githubReleaseMetrics.Describe(ch)
	c.lastScrapeTimestampMetric.Describe(ch)
}
