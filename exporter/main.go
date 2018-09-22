package main

import (
	"github.com/orange-cloudfoundry/githubrelease_exporter/githubrelease"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
	"net/http"
	"os"
)

var (
	configFile = kingpin.Flag(
		"config", "Configuration file path ($GITHUBRELEASE_EXPORTER_CONFIG)",
	).Envar("GITHUBRELEASE_EXPORTER_CONFIG").Default("config.yml").File()

	metricsNamespace = kingpin.Flag(
		"metrics.namespace", "Metrics Namespace ($GITHUBRELEASE_EXPORTER_METRICS_NAMESPACE)",
	).Envar("GITHUBRELEASE_EXPORTER_METRICS_NAMESPACE").Default("credhub").String()

	metricsEnvironment = kingpin.Flag(
		"metrics.environment", "Credhub environment label to be attached to metrics ($GITHUBRELEASE_EXPORTER_METRICS_ENVIRONMENT)",
	).Envar("GITHUBRELEASE_EXPORTER_METRICS_ENVIRONMENT").Required().String()

	listenAddress = kingpin.Flag(
		"web.listen-address", "Address to listen on for web interface and telemetry ($GITHUBRELEASE_EXPORTER_WEB_LISTEN_ADDRESS)",
	).Envar("GITHUBRELEASE_EXPORTER_WEB_LISTEN_ADDRESS").Default(":9362").String()

	metricsPath = kingpin.Flag(
		"web.telemetry-path", "Path under which to expose Prometheus metrics ($GITHUBRELEASE_EXPORTER_WEB_TELEMETRY_PATH)",
	).Envar("GITHUBRELEASE_EXPORTER_WEB_TELEMETRY_PATH").Default("/metrics").String()

	authUsername = kingpin.Flag(
		"web.auth.username", "Username for web interface basic auth ($GITHUBRELEASE_EXPORTER_WEB_AUTH_USERNAME)",
	).Envar("GITHUBRELEASE_EXPORTER_WEB_AUTH_USERNAME").String()

	authPassword = kingpin.Flag(
		"web.auth.password", "Password for web interface basic auth ($GITHUBRELEASE_EXPORTER_WEB_AUTH_PASSWORD)",
	).Envar("GITHUBRELEASE_EXPORTER_WEB_AUTH_PASSWORD").String()

	tlsCertFile = kingpin.Flag(
		"web.tls.cert_file", "Path to a file that contains the TLS certificate (PEM format). If the certificate is signed by a certificate authority, the file should be the concatenation of the server's certificate, any intermediates, and the CA's certificate ($GITHUBRELEASE_EXPORTER_WEB_TLS_CERTFILE)",
	).Envar("GITHUBRELEASE_EXPORTER_WEB_TLS_KEYFILE").ExistingFile()

	tlsKeyFile = kingpin.Flag(
		"web.tls.key_file", "Path to a file that contains the TLS private key (PEM format) ($GITHUBRELEASE_EXPORTER_WEB_TLS_KEYFILE)",
	).Envar("GITHUBRELEASE_EXPORTER_WEB_TLS_KEYFILE").ExistingFile()
)

func init() {
	prometheus.MustRegister(version.NewCollector(*metricsNamespace))
}

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
	handler := prometheus.Handler()

	if *authUsername != "" && *authPassword != "" {
		handler = &basicAuthHandler{
			handler:  prometheus.Handler().ServeHTTP,
			username: *authUsername,
			password: *authPassword,
		}
	}

	return handler
}

func main() {
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("githubrelease_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting githubrelease_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	config := githubrelease.NewConfig(*configFile)
	manager, err := githubrelease.NewManager(*config)
	if err != nil {
		os.Exit(1)
	}
	collector := NewGithubCollector(*metricsEnvironment, *manager)
	prometheus.MustRegister(collector)
	handler := prometheusHandler()
	http.Handle(*metricsPath, handler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>Credhub Exporter</title></head>
             <body>
             <h1>Credhub Exporter</h1>
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
