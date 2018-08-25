package main

import (
	"context"
	"fmt"
	boshtpl "github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/cppforlife/go-patch/patch"
	"github.com/google/go-github/github"
	"github.com/prometheus/common/log"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"regexp"
	"sort"
)

// Manager -
type Manager struct {
	config Config
	client *github.Client
	ctx    context.Context
}

// NewManager -
func NewManager(config Config) Manager {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: config.GithubToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	return Manager{
		config: config,
		client: github.NewClient(tc),
		ctx:    ctx,
	}
}

// GithubRef -
type GithubRef struct {
	Ref  string
	Unix int64
}

// GetRefs -
func (a *Manager) GetRefs(item GithubReleaseConfig) ([]GithubRef, error) {
	res := []GithubRef{}
	release := item.HasType("release")
	preRelease := item.HasType("pre_release")
	draftRelease := item.HasType("draft_release")
	releases, _, err := a.client.Repositories.ListReleases(a.ctx, item.Owner, item.Repo, nil)
	if err != nil {
		return res, fmt.Errorf("unable to fetch releases from %s/%s: %s", item.Owner, item.Repo, err)
	}
	for _, r := range releases {
		if (r.GetPrerelease() == preRelease) ||
			(r.GetDraft() == draftRelease) ||
			(!r.GetPrerelease() && !r.GetDraft() && release) {
			res = append(res, GithubRef{r.GetTagName(), r.GetCreatedAt().Unix()})
		}
	}

	return res, nil
}

// GetLastRef -
func (a *Manager) GetLastRef(refs []GithubRef) (string, error) {
	sort.Slice(refs[:], func(i, j int) bool {
		return refs[i].Unix > refs[j].Unix
	})
	if len(refs) == 0 {
		return "", fmt.Errorf("unable to find any release")
	}
	return refs[0].Ref, nil
}

// FormatVersion -
func (a *Manager) FormatVersion(ref string, item GithubReleaseConfig) string {
	re := regexp.MustCompile(item.Format.Match)
	return re.ReplaceAllString(ref, item.Format.Replace)
}

// GetContent -
func (a *Manager) GetContent(ref string, item BoshDeploymentConfig, path string) ([]byte, error) {
	opts := github.RepositoryContentGetOptions{Ref: ref}
	stream, err := a.client.Repositories.DownloadContents(a.ctx, item.Owner, item.Repo, path, &opts)
	if err != nil {
		return []byte{}, fmt.Errorf("could not download : %s", err)
	}

	defer stream.Close()
	content, err := ioutil.ReadAll(stream)
	if err != nil {
		return []byte{}, fmt.Errorf("could read stream : %s", err)
	}

	return content, nil
}

// GetGithubReleases -
func (a *Manager) GetGithubReleases() []GithubReleaseData {
	results := []GithubReleaseData{}
	for name, item := range a.config.GithubReleases {
		entry := log.
			With("name", name).
			With("repo", item.Repo).
			With("owner", item.Owner)
		entry.Debugf("processing github release")

		results = append(results, NewGithubReleaseData(*item, name))
		target := &results[len(results)-1]

		entry.Debugf("fetching release list")
		refs, err := a.GetRefs(*item)
		if err != nil {
			log.Errorf("skiping github release '%s' for %s/%s : %s", name, item.Owner, item.Repo, err)
			target.HasError = true
			continue
		}

		target.LastRef, err = a.GetLastRef(refs)
		if err != nil {
			log.Errorf("skiping github release '%s' for %s/%s : %s", name, item.Owner, item.Repo, err)
			target.HasError = true
			continue
		}
		target.Version = a.FormatVersion(target.LastRef, *item)
	}
	return results
}

// RenderManifest -
func (a *Manager) RenderManifest(manifest []byte, item BoshDeploymentData) []byte {
	entry := log.
		With("deployment", item.Deployment).
		With("repo", item.Repo).
		With("owner", item.Owner)
	entry.Debugf("rendering final manifest")

	tpl := boshtpl.NewTemplate(manifest)
	var opList patch.Ops
	var opListFinal patch.Ops

	for _, opPath := range item.Ops {
		val, err := a.GetContent(item.LastRef, item.BoshDeploymentConfig, opPath)
		if err != nil {
			item.HasError = true
			entry.Warnf("unable to fetch ops-file '%s'", opPath)
			continue
		}

		var opDef []patch.OpDefinition
		if err = yaml.Unmarshal(val, &opDef); err != nil {
			item.HasError = true
			entry.Warnf("unable to parse ops-file '%s'", opPath)
			continue
		}

		ops, err := patch.NewOpsFromDefinitions(opDef)
		if err != nil {
			item.HasError = true
			entry.Warnf("unable to create ops from file '%s'", opPath)
			continue
		}
		opList = append(opList, ops)
		_, err = tpl.Evaluate(boshtpl.MultiVars{}, opList, boshtpl.EvaluateOpts{})
		if err != nil {
			entry.Warnf("skipping ops file '%s' : %s", opPath, err)
			continue
		}
		opListFinal = append(opListFinal, ops)
	}

	res, err := tpl.Evaluate(boshtpl.MultiVars{}, opListFinal, boshtpl.EvaluateOpts{})
	if err != nil {
		entry.Warnf("enable to render manifest with ops-files : %s", err)
		item.HasError = true
		return manifest
	}

	return res
}

// GetBoshDeployments -
func (a *Manager) GetBoshDeployments() []BoshDeploymentData {
	results := []BoshDeploymentData{}
	for name, item := range a.config.BoshDeployments {
		results = append(results, NewBoshDeploymentData(*item, name))
		target := &results[len(results)-1]

		entry := log.
			With("deployment", name).
			With("repo", item.Repo).
			With("owner", item.Owner)
		entry.Debugf("processing bosh deployment")

		entry.Debugf("fetching release list")
		refs, err := a.GetRefs(item.GithubReleaseConfig)
		if err != nil {
			log.Errorf("skiping bosh deployment '%s' for %s/%s : %s", name, item.Owner, item.Repo, err)
			target.HasError = true
			continue
		}

		target.LastRef, err = a.GetLastRef(refs)
		if err != nil {
			log.Errorf("skiping bosh deployment '%s' for %s/%s : %s", name, item.Owner, item.Repo, err)
			target.HasError = true
			continue
		}
		target.Version = a.FormatVersion(target.LastRef, item.GithubReleaseConfig)

		entry.Debugf("downloading manifest")
		content, err := a.GetContent(target.LastRef, *item, item.Manifest)
		if err != nil {
			log.Errorf("skiping bosh deployment '%s' for %s/%s : %s", name, item.Owner, item.Repo, err)
			target.HasError = true
			continue
		}

		final := a.RenderManifest(content, *target)

		entry.Debugf("extracting bosh-release versions")
		var manifest BoshManifest
		err = yaml.Unmarshal(final, &manifest)
		if err != nil {
			log.Errorf("unable parse manifest '%s' from %s/%s : %s", item.Manifest, item.Owner, item.Repo, err)
			target.HasError = true
			continue
		}
		target.Releases = manifest.Releases
	}
	return results
}
