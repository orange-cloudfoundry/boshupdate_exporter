package boshupdate

import (
	"context"
	"fmt"
	"github.com/cloudfoundry/bosh-cli/director"
	boshtpl "github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/cppforlife/go-patch/patch"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/prometheus/common/log"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"regexp"
	"sort"
	"time"
)

type cache struct {
	ttl        time.Duration
	lastUpdate time.Time
	generics   []GenericReleaseData
	manifests  []ManifestReleaseData
}

// Manager -
type Manager struct {
	config   Config
	client   *github.Client
	ctx      context.Context
	director director.Director
	cache    cache
}

// NewManager -
func NewManager(config Config) (*Manager, error) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: config.Github.Token},
	)
	tc := oauth2.NewClient(ctx, ts)
	director, err := NewDirector(config.Bosh)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to create director client")
	}

	ttl, err := time.ParseDuration(config.Github.UpdateInterval)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid github interval duration '%s'", config.Github.UpdateInterval)
	}

	return &Manager{
		config:   config,
		client:   github.NewClient(tc),
		ctx:      ctx,
		director: director,
		cache: cache{
			ttl:        ttl,
			lastUpdate: time.Unix(0, 0),
			generics:   []GenericReleaseData{},
			manifests:  []ManifestReleaseData{},
		},
	}, nil
}

// GetGenericReleases -
func (a *Manager) GetGenericReleases() []GenericReleaseData {
	if time.Since(a.cache.lastUpdate) > a.cache.ttl {
		a.cache.generics = a.updateGenericReleases()
		a.cache.manifests = a.updateManifestReleases()
		a.cache.lastUpdate = time.Now()
	}
	return a.cache.generics
}

// GetManifestReleases -
func (a *Manager) GetManifestReleases() []ManifestReleaseData {
	if time.Since(a.cache.lastUpdate) > a.cache.ttl {
		a.cache.generics = a.updateGenericReleases()
		a.cache.manifests = a.updateManifestReleases()
		a.cache.lastUpdate = time.Now()
	}
	return a.cache.manifests
}

// GetBoshDeployments -
func (a *Manager) GetBoshDeployments() ([]BoshDeploymentData, error) {
	entry := log.With("name", "deployments")
	entry.Debugf("processing bosh deployments")

	res := []BoshDeploymentData{}
	re := regexp.MustCompile("v(.*)")

	deployments, err := a.director.Deployments()
	if err != nil {
		return res, errors.Wrapf(err, "unable to fetch deployments")
	}

	for _, deployment := range deployments {
		entry.Debugf("processing bosh deployment %s", deployment.Name())
		manifest, err := deployment.Manifest()
		if err != nil {
			log.Errorf("unable to fetch manifest for deployment '%s': %+v", deployment.Name(), err)
			res = append(res, BoshDeploymentData{
				Deployment: deployment.Name(),
				HasError:   true,
			})
			continue
		}

		data := struct {
			Version  string        `yaml:"manifest_version"`
			Name     string        `yaml:"manifest_name"`
			Releases []BoshRelease `yaml:"releases"`
		}{}
		err = yaml.Unmarshal([]byte(manifest), &data)
		if err != nil {
			log.Errorf("unable to parse manifest for deployment '%s': %+v", deployment.Name(), err)
			res = append(res, BoshDeploymentData{
				Deployment: deployment.Name(),
				HasError:   true,
			})
		}

		if data.Name == "" {
			data.Name = deployment.Name()
		}
		if a.config.Bosh.IsExcluded(data.Name) {
			log.Debugf("excluding deployment '%s'", data.Name)
			continue
		}

		if data.Version == "" {
			log.Errorf("unable to find manifest version for deployment '%s'", deployment.Name())
			res = append(res, BoshDeploymentData{
				Deployment: deployment.Name(),
				HasError:   true,
			})
			continue
		}

		if re.MatchString(data.Version) {
			data.Version = re.ReplaceAllString(data.Version, "${1}")
		}

		res = append(res, BoshDeploymentData{
			Deployment:   deployment.Name(),
			ManifestName: data.Name,
			Ref:          data.Version,
			HasError:     false,
			BoshReleases: data.Releases,
		})
	}
	return res, nil
}

func (a *Manager) updateGenericReleases() []GenericReleaseData {
	results := []GenericReleaseData{}
	for name, item := range a.config.Github.GenericReleases {
		entry := log.
			With("name", name).
			With("repo", item.Repo).
			With("owner", item.Owner)
		entry.Debugf("processing github release")

		results = append(results, NewGenericReleaseData(*item, name))
		target := &results[len(results)-1]

		entry.Debugf("fetching release list")
		refs, err := a.getRefs(*item)
		if err != nil {
			entry.Errorf("skiping generic release: %+v", err)
			target.HasError = true
			continue
		}

		lastRef, err := a.getLastRef(refs)
		if err != nil {
			entry.Errorf("skiping generic release: %+v", err)
			target.HasError = true
			continue
		}

		target.Versions = a.createVersions(refs, *lastRef, *item)
		target.LatestVersion = NewVersion(lastRef.Ref, item.Format.Format(lastRef.Ref), lastRef.Time)
	}
	return results
}

func (a *Manager) updateManifestReleases() []ManifestReleaseData {
	results := []ManifestReleaseData{}

	for name, item := range a.config.Github.ManifestReleases {
		results = append(results, NewManifestReleaseData(*item, name))
		target := &results[len(results)-1]

		entry := log.
			With("deployment", name).
			With("repo", item.Repo).
			With("owner", item.Owner)
		entry.Debugf("processing bosh deployment")

		entry.Debugf("fetching release list")
		refs, err := a.getRefs(item.GenericReleaseConfig)
		if err != nil {
			entry.Errorf("skiping manifest release: %+v", err)
			target.HasError = true
			continue
		}

		lastRef, err := a.getLastRef(refs)
		if err != nil {
			entry.Errorf("skiping manifest release: %+v", err)
			target.HasError = true
			continue
		}
		target.Versions = a.createVersions(refs, *lastRef, item.GenericReleaseConfig)
		target.LatestVersion = NewVersion(lastRef.Ref, item.Format.Format(lastRef.Ref), lastRef.Time)

		entry = log.
			With("deployment", name).
			With("repo", item.Repo).
			With("owner", item.Owner).
			With("version", target.LatestVersion.Version)

		if len(item.Manifest) == 0 {
			continue
		}

		entry.Debugf("downloading manifest")
		content, err := a.getContent(lastRef.Ref, *item, item.Manifest)
		if err != nil {
			entry.Warnf("skiping manifest release: %+v", err)
			continue
		}

		final, err := a.RenderManifest(content, *target)
		if err != nil {
			entry.Warnf("skiping manifest release: %+v", err)
			continue
		}

		entry.Debugf("extracting bosh-release versions")
		var manifest BoshManifest
		err = yaml.Unmarshal(final, &manifest)
		if err != nil {
			entry.Warnf("unable to parse manifest '%s': %+v", item.Manifest, err)
			continue
		}
		target.BoshReleases = manifest.Releases
	}
	return results
}

// 1. For some reason, we get empty date when reading tag object
//    We fetch information for corresponding sha to get tag date
func (a *Manager) getRefs(item GenericReleaseConfig) ([]GithubRef, error) {
	res := []GithubRef{}
	release := item.HasType("release")
	preRelease := item.HasType("pre_release")
	draftRelease := item.HasType("draft_release")
	releases, _, err := a.client.Repositories.ListReleases(a.ctx, item.Owner, item.Repo, nil)
	if err != nil {
		return res, errors.Wrapf(err, "unable to fetch releases from %s/%s", item.Owner, item.Repo)
	}
	for _, r := range releases {
		if (r.GetPrerelease() == preRelease) ||
			(r.GetDraft() == draftRelease) ||
			(!r.GetPrerelease() && !r.GetDraft() && release) {
			res = append(res, GithubRef{r.GetTagName(), r.GetCreatedAt().Unix()})
		}
	}

	if item.HasType("tag") {
		tags, _, err := a.client.Repositories.ListTags(a.ctx, item.Owner, item.Repo, nil)
		if err != nil {
			return res, errors.Wrapf(err, "unable to fetch tags from %s/%s", item.Owner, item.Repo)
		}
		for _, t := range tags {
			// 1.
			sha1 := t.GetCommit().GetSHA()
			commit, _, _ := a.client.Repositories.GetCommit(a.ctx, item.Owner, item.Repo, sha1)
			res = append(res, GithubRef{t.GetName(), commit.GetCommit().GetCommitter().GetDate().Unix()})
		}
	}

	sort.Slice(res[:], func(i, j int) bool {
		return res[i].Time > res[j].Time
	})
	return res, nil
}

func (a *Manager) getLastRef(refs []GithubRef) (*GithubRef, error) {
	if len(refs) == 0 {
		return nil, fmt.Errorf("unable to find any release")
	}
	return &refs[0], nil
}

func (a *Manager) getContent(ref string, item ManifestReleaseConfig, path string) ([]byte, error) {
	opts := github.RepositoryContentGetOptions{Ref: ref}
	stream, err := a.client.Repositories.DownloadContents(a.ctx, item.Owner, item.Repo, path, &opts)
	if err != nil {
		return []byte{}, errors.Wrapf(err, "could not download file '%s'", path)
	}

	defer stream.Close()
	content, err := ioutil.ReadAll(stream)
	if err != nil {
		return []byte{}, errors.Wrapf(err, "could read remote stream")
	}

	return content, nil
}

func (a *Manager) createVersions(refs []GithubRef, last GithubRef, item GenericReleaseConfig) []Version {
	versions := []Version{}
	for idx, ref := range refs {
		v := NewVersion(ref.Ref, item.Format.Format(ref.Ref), ref.Time)
		if idx != 0 {
			v.ExpiredSince = refs[idx-1].Time
		}
		versions = append(versions, v)
	}
	return versions
}

// RenderManifest -
func (a *Manager) RenderManifest(manifest []byte, item ManifestReleaseData) ([]byte, error) {
	entry := log.
		With("name", item.Name).
		With("repo", item.Repo).
		With("owner", item.Owner)
	entry.Debugf("rendering final manifest")

	tpl := boshtpl.NewTemplate(manifest)

	var opList patch.Ops
	var opListFinal patch.Ops
	for _, opPath := range item.Ops {
		val, err := a.getContent(item.LatestVersion.GitRef, item.ManifestReleaseConfig, opPath)
		if err != nil {
			return []byte{}, errors.Wrapf(err, "unable to fetch ops-file '%s'", opPath)
		}
		var opDef []patch.OpDefinition
		if err = yaml.Unmarshal(val, &opDef); err != nil {
			return []byte{}, errors.Wrapf(err, "unable to parse ops-file '%s'", opPath)
		}
		ops, err := patch.NewOpsFromDefinitions(opDef)
		if err != nil {
			return []byte{}, errors.Wrapf(err, "unable to create ops from file '%s'", opPath)
		}
		opList = append(opList, ops)
		_, err = tpl.Evaluate(boshtpl.MultiVars{}, opList, boshtpl.EvaluateOpts{})
		if err != nil {
			return []byte{}, errors.Wrapf(err, "unable to evaluate ops file '%s'", opPath)
		}
		opListFinal = append(opListFinal, ops)
	}

	varList := []boshtpl.Variables{}
	for _, varPath := range item.Vars {
		val, err := a.getContent(item.LatestVersion.GitRef, item.ManifestReleaseConfig, varPath)
		if err != nil {
			return []byte{}, errors.Wrapf(err, "unable to fetch var-file '%s'", varPath)
		}
		vars := boshtpl.StaticVariables{}
		if err = yaml.Unmarshal(val, &vars); err != nil {
			return []byte{}, errors.Wrapf(err, "unable to parse var-file '%s'", varPath)
		}
		varList = append(varList, vars)
	}

	res, err := tpl.Evaluate(boshtpl.NewMultiVars(varList), opListFinal, boshtpl.EvaluateOpts{})
	if err != nil {
		return []byte{}, errors.Wrapf(err, "enable to render manifest with ops-files")
	}

	return res, nil
}

// Local Variables:
// ispell-local-dictionary: "american"
// End:
