package githubrelease

import (
	"context"
	"fmt"
	"github.com/cloudfoundry/bosh-cli/director"
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
	config   Config
	client   *github.Client
	ctx      context.Context
	director director.Director
}

// NewManager -
func NewManager(config Config) (*Manager, error) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: config.GithubToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	director, err := NewDirector(config.Bosh)
	if err != nil {
		return nil, fmt.Errorf("unable to create director client : %s", err)
	}

	return &Manager{
		config:   config,
		client:   github.NewClient(tc),
		ctx:      ctx,
		director: director,
	}, nil
}

// GetManifests -
func (a *Manager) GetManifests() ([]ManifestData, error) {
	entry := log.With("name", "deployments")
	entry.Debugf("processing bosh deployments")

	res := []ManifestData{}
	re := regexp.MustCompile("v(.*)")

	deployments, err := a.director.Deployments()
	if err != nil {
		return res, fmt.Errorf("unable to fetch deployments : %s", err)
	}

	for _, deployment := range deployments {
		entry.Debugf("processing bosh deployment %s", deployment.Name())
		manifest, err := deployment.Manifest()
		if err != nil {
			log.Errorf("unable to fetch manifest for deployment '%s' : %s", deployment.Name(), err)
			res = append(res, ManifestData{
				Deployment: deployment.Name(),
				HasError:   true,
			})
			continue
		}

		data := struct {
			Version string `yaml:"manifest_version"`
			Name    string `yaml:"manifest_name"`
		}{}
		err = yaml.Unmarshal([]byte(manifest), &data)
		if err != nil {
			log.Errorf("unable to parse manifest for deployment '%s' : %s", deployment.Name(), err)
			res = append(res, ManifestData{
				Deployment: deployment.Name(),
				HasError:   true,
			})
		}
		if data.Name == "" {
			data.Name = deployment.Name()
		}
		if re.MatchString(data.Version) {
			data.Version = re.ReplaceAllString(data.Version, "${1}")
		}
		res = append(res, ManifestData{
			Deployment: deployment.Name(),
			Name:       data.Name,
			Version:    data.Version,
			HasError:   false,
		})
	}
	return res, nil
}

// GetRefs -
// 1. For some reason, we get empty date when reading tag object
//    We fetch information for corresponding sha to get tag date
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

	if item.HasType("tag") {
		tags, _, err := a.client.Repositories.ListTags(a.ctx, item.Owner, item.Repo, nil)
		if err != nil {
			return res, fmt.Errorf("unable to fetch tags from %s/%s: %s", item.Owner, item.Repo, err)
		}
		for _, t := range tags {
			// 1.
			sha1 := t.GetCommit().GetSHA()
			commit, _, _ := a.client.Repositories.GetCommit(a.ctx, item.Owner, item.Repo, sha1)
			res = append(res, GithubRef{t.GetName(), commit.GetCommit().GetCommitter().GetDate().Unix()})
		}
	}
	return res, nil
}

// GetLastRef -
func (a *Manager) GetLastRef(refs []GithubRef) (*GithubRef, error) {
	sort.Slice(refs[:], func(i, j int) bool {
		return refs[i].Unix > refs[j].Unix
	})
	if len(refs) == 0 {
		return nil, fmt.Errorf("unable to find any release")
	}
	return &refs[0], nil
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
	for name, item := range a.config.GithubRelease {
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

		lastRef, err := a.GetLastRef(refs)
		if err != nil {
			log.Errorf("skiping github release '%s' for %s/%s : %s", name, item.Owner, item.Repo, err)
			target.HasError = true
			continue
		}
		target.LastRef = lastRef.Ref
		target.Time = lastRef.Unix
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

	varList := []boshtpl.Variables{}
	for _, varPath := range item.Vars {
		val, err := a.GetContent(item.LastRef, item.BoshDeploymentConfig, varPath)
		if err != nil {
			item.HasError = true
			entry.Warnf("unable to fetch var-file '%s'", varPath)
			continue
		}
		vars := boshtpl.StaticVariables{}
		if err = yaml.Unmarshal(val, &vars); err != nil {
			item.HasError = true
			entry.Warnf("unable to parse var-file '%s'", varPath)
			continue
		}
		varList = append(varList, vars)
	}

	res, err := tpl.Evaluate(boshtpl.NewMultiVars(varList), opListFinal, boshtpl.EvaluateOpts{})
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
	for name, item := range a.config.BoshDeployment {
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

		lastRef, err := a.GetLastRef(refs)
		if err != nil {
			log.Errorf("skiping bosh deployment '%s' for %s/%s : %s", name, item.Owner, item.Repo, err)
			target.HasError = true
			continue
		}
		target.LastRef = lastRef.Ref
		target.Time = lastRef.Unix
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

// Local Variables:
// ispell-local-dictionary: "american"
// End:
