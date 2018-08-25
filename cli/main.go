package main

import (
	"encoding/json"
	"fmt"
	"github.com/orange-cloudfoundry/githubrelease_exporter/githubrelease"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v2"
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

	manager := githubrelease.NewManager(*config)
	results := manager.GetBoshDeployments()
	var content []byte
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
