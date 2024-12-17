package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/orange-cloudfoundry/boshupdate_exporter/boshupdate"
	"github.com/prometheus/common/version"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

var (
	configFile = kingpin.Flag("config", "Configuration file path").Required().File()
	logLevel   = kingpin.Flag(
		"log.level", "Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal]",
	).Default("info").String()
	logStream = kingpin.Flag(
		"log.stream", "Write log to given stream. Valid streams: [stdout, stderr]",
	).Default("stderr").String()
	logJson = kingpin.Flag(
		"log.json", "When given, write log in json format",
	).Bool()
)

func main() {
	kingpin.Version(version.Print("boshupdate_cli"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.SetLevel(log.ErrorLevel)
	if lvl, err := log.ParseLevel(*logLevel); err == nil {
		log.SetLevel(lvl)
	}
	log.SetOutput(os.Stderr)
	if *logStream == "stdout" {
		log.SetOutput(os.Stdout)
	}
	if *logJson {
		log.SetFormatter(&log.JSONFormatter{})
	}
	config := boshupdate.NewConfig(*configFile)

	var content []byte
	manager, err := boshupdate.NewManager(*config)
	if err != nil {
		log.Errorf("unable to start exporter : %s", err)
		os.Exit(1)
	}

	manifests := manager.GetManifestReleases()
	content, _ = yaml.Marshal(manifests)
	fmt.Println("fetched manifest releases:")
	fmt.Println(string(content))

	generic := manager.GetGenericReleases()
	content, _ = yaml.Marshal(generic)
	fmt.Println("fetched generic releases:")
	fmt.Println(string(content))

	deployments, err := manager.GetBoshDeployments()
	if err != nil {
		log.Errorf("unable to fetch deployments : %s", err)
		os.Exit(1)
	}
	content, _ = yaml.Marshal(deployments)
	fmt.Println("fetched deployments:")
	fmt.Println(string(content))
}
