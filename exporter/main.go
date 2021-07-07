package main

import (
	"github.com/orange-cloudfoundry/boshupdate_exporter/boshupdate"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
	"net/http"
	"os"
	"time"
)

var (
	configFile = kingpin.Flag(
		"config", "Configuration file path ($BOSHUPDATE_EXPORTER_CONFIG)",
	).Envar("BOSHUPDATE_EXPORTER_CONFIG").Default("config.yml").File()

	metricsNamespace = kingpin.Flag(
		"metrics.namespace", "Metrics Namespace ($BOSHUPDATE_EXPORTER_METRICS_NAMESPACE)",
	).Envar("BOSHUPDATE_EXPORTER_METRICS_NAMESPACE").Default("boshupdate").String()

	metricsEnvironment = kingpin.Flag(
		"metrics.environment", "Boshupdate environment label to be attached to metrics ($BOSHUPDATE_EXPORTER_METRICS_ENVIRONMENT)",
	).Envar("BOSHUPDATE_EXPORTER_METRICS_ENVIRONMENT").Required().String()

	listenAddress = kingpin.Flag(
		"web.listen-address", "Address to listen on for web interface and telemetry ($BOSHUPDATE_EXPORTER_WEB_LISTEN_ADDRESS)",
	).Envar("BOSHUPDATE_EXPORTER_WEB_LISTEN_ADDRESS").Default(":9362").String()

	metricsPath = kingpin.Flag(
		"web.telemetry-path", "Path under which to expose Prometheus metrics ($BOSHUPDATE_EXPORTER_WEB_TELEMETRY_PATH)",
	).Envar("BOSHUPDATE_EXPORTER_WEB_TELEMETRY_PATH").Default("/metrics").String()

	authUsername = kingpin.Flag(
		"web.auth.username", "Username for web interface basic auth ($BOSHUPDATE_EXPORTER_WEB_AUTH_USERNAME)",
	).Envar("BOSHUPDATE_EXPORTER_WEB_AUTH_USERNAME").String()

	authPassword = kingpin.Flag(
		"web.auth.password", "Password for web interface basic auth ($BOSHUPDATE_EXPORTER_WEB_AUTH_PASSWORD)",
	).Envar("BOSHUPDATE_EXPORTER_WEB_AUTH_PASSWORD").String()

	tlsCertFile = kingpin.Flag(
		"web.tls.cert_file", "Path to a file that contains the TLS certificate (PEM format). If the certificate is signed by a certificate authority, the file should be the concatenation of the server's certificate, any intermediates, and the CA's certificate ($BOSHUPDATE_EXPORTER_WEB_TLS_CERTFILE)",
	).Envar("BOSHUPDATE_EXPORTER_WEB_TLS_KEYFILE").ExistingFile()

	tlsKeyFile = kingpin.Flag(
		"web.tls.key_file", "Path to a file that contains the TLS private key (PEM format) ($BOSHUPDATE_EXPORTER_WEB_TLS_KEYFILE)",
	).Envar("BOSHUPDATE_EXPORTER_WEB_TLS_KEYFILE").ExistingFile()

	logLevel = kingpin.Flag(
		"log.level", "Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal]",
	).Default("info").String()
	logStream = kingpin.Flag(
		"log.stream", "Write log to given stream. Valid streams: [stdout, stderr]",
	).Default("stderr").String()
	logJson = kingpin.Flag(
		"log.json", "When given, write log in json format",
	).Bool()
)

type basicAuthHandler struct {
	handler  http.HandlerFunc
	username string
	password string
}

func (h *basicAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	username, password, ok := r.BasicAuth()
	if !ok || username != h.username || password != h.password {
		log.Errorf("Invalid HTTP auth from `%s`", r.RemoteAddr)
		w.Header().Set("WWW-Authenticate", "Basic realm=\"metrics\"")
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}
	h.handler(w, r)
}

func prometheusHandler() http.Handler {
	handler := promhttp.Handler()
	if *authUsername != "" && *authPassword != "" {
		handler = &basicAuthHandler{
			handler:  promhttp.Handler().ServeHTTP,
			username: *authUsername,
			password: *authPassword,
		}
	}
	return handler
}

func main() {
	kingpin.Version(version.Print("boshupdate_exporter"))
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

	log.Infoln("Starting boshupdate_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	config := boshupdate.NewConfig(*configFile)
	manager, err := boshupdate.NewManager(*config)
	if err != nil {
		log.Errorln(err)
		os.Exit(1)
	}

	initMetricsReporter(*metricsNamespace, *metricsEnvironment)

	interval, _ := time.ParseDuration(config.Github.UpdateInterval)
	startUpdate(manager, interval)
	http.Handle(*metricsPath, prometheusHandler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>Boshupdate Exporter</title></head>
             <body>
             <h1>Boshupdate Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})

	if *tlsCertFile != "" && *tlsKeyFile != "" {
		log.Infoln("Listening TLS on", *listenAddress)
		log.Fatal(http.ListenAndServeTLS(*listenAddress, *tlsCertFile, *tlsKeyFile, nil))
	} else {
		log.Infoln("Listening on", *listenAddress)
		log.Fatal(http.ListenAndServe(*listenAddress, nil))
	}
}
