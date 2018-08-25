package main

// import (
// 	"encoding/json"
// 	"fmt"
// 	log "github.com/sirupsen/logrus"
// 	"gopkg.in/alecthomas/kingpin.v2"
// 	"gopkg.in/yaml.v2"
// 	"os"
// )

// var (
// 	version    = "0.1.0"
// 	configFile = kingpin.Flag(
// 		"config", "Configuration file path",
// 	).Required().File()
// 	doYaml = kingpin.Flag(
// 		"yaml", "Generate yaml output instead of json",
// 	).Bool()
// )

// func main() {
// 	kingpin.Version(version)
// 	kingpin.HelpFlag.Short('h')
// 	kingpin.Parse()

// 	log.SetOutput(os.Stderr)
// 	log.SetLevel(log.ErrorLevel)
// 	config := NewConfig(*configFile)
// 	if lvl, err := log.ParseLevel(config.Log.Level); err == nil {
// 		log.SetLevel(lvl)
// 	}
// 	if config.Log.JSON {
// 		log.SetFormatter(&log.JSONFormatter{})
// 	}

// 	manager := NewManager(*config)
// 	results := manager.GetBoshDeployments()
// 	var content []byte
// 	if *doYaml {
// 		content, _ = yaml.Marshal(results)
// 	} else {
// 		content, _ = json.Marshal(results)
// 	}
// 	fmt.Println(string(content))

// 	results2 := manager.GetGithubReleases()
// 	if *doYaml {
// 		content, _ = yaml.Marshal(results2)
// 	} else {
// 		content, _ = json.Marshal(results2)
// 	}
// 	fmt.Println(string(content))
// }
