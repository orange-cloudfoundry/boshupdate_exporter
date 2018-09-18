package main

import (
	"encoding/json"
	"fmt"
	"github.com/orange-cloudfoundry/githubrelease_exporter/githubrelease"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v2"
	"os"
)

var (
	configFile = kingpin.Flag("config", "Configuration file path").Required().File()
	doYaml     = kingpin.Flag("yaml", "Generate yaml output instead of json").Bool()
)

func main() {
	kingpin.Version(version.Print("githubrelease_cli"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Base().SetFormat("logger://stderr")
	log.Base().SetLevel("error")

	config := githubrelease.NewConfig(*configFile)
	log.Base().SetLevel(config.Log.Level)
	if config.Log.JSON {
		log.Base().SetFormat("logger://stderr?json=true")
	}

	manager, err := githubrelease.NewManager(*config)
	if err != nil {
		log.Errorf("unable to start exporter : %s", err)
		os.Exit(1)
	}

	var content []byte
	manifests, err := manager.GetManifests()
	if err != nil {
		os.Exit(1)
	}
	if *doYaml {
		content, _ = yaml.Marshal(manifests)
	} else {
		content, _ = json.Marshal(manifests)
	}
	fmt.Println(string(content))

	results := manager.GetBoshDeployments()
	if *doYaml {
		content, _ = yaml.Marshal(results)
	} else {
		content, _ = json.Marshal(results)
	}
	fmt.Println(string(content))

	results2 := manager.GetGithubReleases()
	if *doYaml {
		content, _ = yaml.Marshal(results2)
	} else {
		content, _ = json.Marshal(results2)
	}
	fmt.Println(string(content))
}
