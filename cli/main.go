package main

import (
	"fmt"
	"github.com/orange-cloudfoundry/boshupdate_exporter/boshupdate"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v2"
	"os"
)

var (
	configFile = kingpin.Flag("config", "Configuration file path").Required().File()
)

func main() {
	kingpin.Version(version.Print("boshupdate_cli"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Base().SetFormat("logger://stderr")
	log.Base().SetLevel("error")

	config := boshupdate.NewConfig(*configFile)
	log.Base().SetLevel(config.Log.Level)
	if config.Log.JSON {
		log.Base().SetFormat("logger://stderr?json=true")
	}

	var content []byte
	manager, err := boshupdate.NewManager(*config)
	if err != nil {
		log.Errorf("unable to start exporter : %s", err)
		os.Exit(1)
	}

	manifests := manager.GetManifestReleases()
	content, _ = yaml.Marshal(manifests)
	fmt.Println(string(content))

	// generic := manager.GetGenericReleases()
	// content, _ = yaml.Marshal(generic)
	// fmt.Println(string(content))

	deployments, err := manager.GetBoshDeployments()
	if err != nil {
		log.Errorf("unable to fetch deployments : %s", err)
		os.Exit(1)
	}
	content, _ = yaml.Marshal(deployments)
	fmt.Println(string(content))
}
