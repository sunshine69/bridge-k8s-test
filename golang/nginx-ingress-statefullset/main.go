package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"os"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/klauspost/compress/gzhttp"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"gopkg.in/yaml.v2"
)

type ServerConfig struct {
	SharedToken  string
	Port         int
	Serverdomain string
}

var (
	Config                   ServerConfig
	version, sslKey, sslCert string
)

func homePage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello. If you see this message, the auth works")
	fmt.Println("Endpoint Hit: homePage. ")
}

func ContainerStatus(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

func GetVersion(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, version)
}

//HandleRequests -
func HandleRequests() {
	router := mux.NewRouter()
	router.Handle("/", isAuthorized(homePage)).Methods("GET")

	router.Handle("/version", isAuthorized(GetVersion)).Methods("GET")
	//k8s container probe
	router.HandleFunc("/container_status", ContainerStatus).Methods("GET")
	//Debugging

	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", Config.Port),
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      router, // Pass our instance of gorilla/mux in.
	}
	srv.Handler = gzhttp.GzipHandler(router)

	if sslKey == "" {
		log.Printf("Start server on port %d\n", Config.Port)
		log.Fatal(srv.ListenAndServe())
	} else {
		if sslKey == "auto" {

			fmt.Printf("PORT %d\n", Config.Port)
			client := &acme.Client{DirectoryURL: autocert.DefaultACMEDirectory}
			// client := &acme.Client{DirectoryURL: "https://acme-staging-v02.api.letsencrypt.org/directory" }
			certManager := autocert.Manager{
				Prompt:     autocert.AcceptTOS,
				Cache:      autocert.DirCache("certs"),
				HostPolicy: autocert.HostWhitelist(Config.Serverdomain),
				Client:     client,
			}
			srv.TLSConfig = &tls.Config{
				GetCertificate: certManager.GetCertificate,
			}
			go http.ListenAndServe(":80", certManager.HTTPHandler(nil))
			log.Printf("Start SSL/TLS server with letsencrypt enabled on port %d\n", Config.Port)
			log.Fatal(srv.ListenAndServeTLS("", ""))
		} else {
			log.Printf("Start SSL/TLS server on port %d\n", Config.Port)
			log.Fatal(srv.ListenAndServeTLS(sslCert, sslKey))
		}
	}
}
func CheckErr(err error, location string) bool {
	if err != nil {
		log.Fatalf("ERROR at %s - %v\n", location, err)
		return false
	} else {
		return true
	}
}
func LoadConfig(fromYaml string) {
	if fromYaml != "" {
		inputDataByte, err := ioutil.ReadFile(fromYaml)
		CheckErr((err), "LoadConfig")
		yamlObj := make(map[string]interface{}, 1)
		yaml.Unmarshal(inputDataByte, &yamlObj)
		for key, val := range yamlObj {
			if _val, ok := val.(string); ok {
				if err := os.Setenv(key, _val); err != nil {
					log.Fatalf("ERROR can not set env vars from yaml config file %v\n", err)
				}
			} else {
				log.Fatalf("ERROR key %s not set properly. It needs to be non empty and string type. Check your config file", key)
			}
		}
	}
	Config.Serverdomain = os.Getenv("SERVER_DOMAIN")
	if _Port, err := strconv.Atoi(os.Getenv("PORT")); err == nil {
		Config.Port = _Port
	} else {
		Config.Port = 8080
	}
}

func main() {
	var yamlConfigMapFile string
	flag.StringVar(&yamlConfigMapFile, "config", "", "Config map file. Used when running manually rather than in K8S. It should read the same configmap file used by the common chart helm system and have the same effect")
	flag.StringVar(&sslKey, "sslKey", "", "ssl key file path. Default empty which wont use SSL. value 'auto' will enable auto cert using lets encrypt where port 80 must be free. If point to key file then option sslCert must be provided as well")
	flag.StringVar(&sslCert, "sslCert", "", "ssl key file path. Default empty which wont use SSL. value 'auto' will enable auto cert using lets encrypt where port 80 must be free. If point to key file then option sslCert must be provided as well")
	flag.Parse()
	LoadConfig(yamlConfigMapFile)
	HandleRequests()
}

func isAuthorized(endpoint func(http.ResponseWriter, *http.Request)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// tokens := r.Header["X-Gitlab-Token"]
		// if len(tokens) > 0 && tokens[0] == Config.SharedToken {
		endpoint(w, r)
		// } else {
		// fmt.Fprintf(w, "Not Authorized")
		// }
	})
}
