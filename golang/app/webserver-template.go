package main

import (
	"strconv"
	"regexp"
	"strings"
	"crypto/tls"
	"database/sql"
	"flag"
	"fmt"
	"github.com/klauspost/compress/gzhttp"
	_ "github.com/mattn/go-sqlite3"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
	"github.com/gorilla/mux"
	u "github.com/sunshine69/golang-tools/utils"
	"github.com/mileusna/crontab"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"gopkg.in/yaml.v2"
)

type ServerConfig struct {
	SharedToken  string
	Port         string
	Serverdomain string
	SslKey       string
	SslCert      string
	Logdbpath    string
	Dbtimeout    string
	LogRetention string
}

var (
	Config  ServerConfig
	version string
)

func LoadConfig(fromYaml string) {
	if fromYaml != "" {
		inputDataByte, err := ioutil.ReadFile(fromYaml)
		u.CheckErr((err), "LoadConfig")
		yamlObj := make(map[string]interface{}, 1)
		u.CheckErr(yaml.Unmarshal(inputDataByte, &yamlObj), "LoadConfig Unmarshal")
		for key, val := range yamlObj {
			if _val, ok := val.(string); ok {
				log.Printf("Setenv %s=%s\n", key, _val)
				if err := os.Setenv(key, _val); err != nil {
					panic(fmt.Sprintf("[ERROR] can not set env vars from yaml config file %v\n", err))
				}
			} else {
				panic(fmt.Sprintf("[ERROR] key %s not set properly. It needs to be non empty and string type. Check your config file", key))
			}
		}
	}
	fmt.Printf("LOG_LEVEL: %s\n", os.Getenv("LOG_LEVEL"))
	u.ConfigureLogging(os.Stdout)
	Config.Logdbpath = os.Getenv("DB_PATH")
	Config.Port = u.Getenv("SERVER_PORT", "8080")
	Config.SharedToken = u.Getenv("SHARED_TOKEN", "changeme")
	Config.SslCert = os.Getenv("SSL_CERT")
	Config.SslKey = os.Getenv("SSL_KEY")
	Config.Serverdomain = os.Getenv("SERVER_DOMAIN")
	Config.Dbtimeout = os.Getenv("DB_TIMEOUT")
	Config.LogRetention = u.Getenv("LOG_RETENTION", "90d")
	log.Printf("[DEBUG] ServerConfig %v\n", Config)
}

func homePage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello. If you see this message, the auth works")
	log.Printf("[DEBUG] This should not print if LOG_LEVEL=ERROR\n")
	log.Printf("[ERROR] This should print LOG_LEVEL=ERROR\n")
}

func ContainerStatus(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

func GetVersion(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, version)
}
func DownloadDB(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(Config.Logdbpath))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, Config.Logdbpath)
}
func BackupAndDropDB(w http.ResponseWriter, r *http.Request) {
	newFileName := fmt.Sprintf("%s.backup", Config.Logdbpath)
	_ = os.Remove(newFileName)
	u.CheckErr(os.Rename(Config.Logdbpath, newFileName), "BackupAndDropDB RENAME FILE")
	SetUpLogDatabase()
	w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(newFileName))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, newFileName)
}
func RunDSL(w http.ResponseWriter, r *http.Request) {
	dbc := GetDBConn()
	defer dbc.Close()
	sql := r.FormValue("sql")

	result, err := dbc.Exec(sql)
	if err != nil {
		fmt.Fprintf(w, `{"error": "RunDSL", "message": "%v"}`, err)
		return
	}
	fmt.Fprintf(w, `{"error": "", "message": "%v"}`, result)

}
func RunSQL(w http.ResponseWriter, r *http.Request) {
	dbc := GetDBConn()
	defer dbc.Close()
	sql := r.FormValue("sql")
	var result = make([]interface{}, 0)

	if (! strings.HasPrefix(sql, "select")) && (! strings.HasPrefix(sql, "SELECT")) {
		fmt.Fprint(w, `{"error": "RunSQL","message":"query not started with SELECT or select, aborting"}`)
		return
	}
	ptn := regexp.MustCompile(`[\s]+(from|FROM)[\s]+([^\s]+)[\s]*`)
	if matches := ptn.FindStringSubmatch(sql); len(matches) == 3 {
		tableName := matches[2]
		rows, err := dbc.Query(sql)
		u.CheckErrNonFatal(err, "RunSQL")
		columnNames, err := rows.Columns() // []string{"id", "name"}
		u.CheckErr(err, "RunSQL columnNames")
		columns := make([]interface{}, len(columnNames))
		columnTypes, _ := rows.ColumnTypes()
		columnPointers := make([]interface{}, len(columnNames))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}
		for rows.Next() {
			err := rows.Scan(columnPointers...)
			u.CheckErrNonFatal(err, "run query error")
			_temp := make(map[string]interface{})
			for idx, _cName := range columnNames {
				if strings.ToUpper(columnTypes[idx].DatabaseTypeName()) == "TEXT" {
					//avoid auto base64 enc by golang when hitting the []byte type
					//not sure why some TEXT return []uint8 other return as string.
					_data, ok := columns[idx].([]uint8)
					if ok {
						_temp[_cName] = string(_data)
					} else {
						_temp[_cName] = columns[idx]
					}
				} else {
					_temp[_cName] = columns[idx]
				}
			}
			result = append(result, _temp)
		}
		fmt.Fprint(w, u.JsonDump(map[string]interface{} {
			tableName: result,
		}, "    ") )
	} else {
		fmt.Fprintf(w, `{"error": "RunSQL", "message": "ERROR Malformed sql, no table name found"}`)
		return
	}

}
/* Sample curl command to save log
curl -k -X POST -H "X-Webserver-Template-Token: changeme" -H "Content-Type: application/x-www-form-urlencoded" 'http://localhost:8080/savelog' --data-urlencode 'logfile={"event": "started", "file": "codeception.yml", "error_code": -1}' --data-urlencode "message='test'" --data-urlencode "application='test app'"
*/
func SaveLog(w http.ResponseWriter, r *http.Request) {
	dbc := GetDBConn()
	defer dbc.Close()

	host := r.FormValue("host")
	application := r.FormValue("application")
	message := r.FormValue("message")
	logfile := r.FormValue("logfile")
	datelog := r.FormValue("datelog")

	if message == "" {
		fmt.Fprintf(w, "ERROR message empty")
		return
	}
	if datelog == "" {
		stmt, err := dbc.Prepare(`INSERT INTO log(host, application, message, logfile) VALUES(?, ?, ?, ?)`)
		u.CheckErr(err, "SaveLog Prepare")
		_, err = stmt.Exec(host, application, message, logfile)
		u.CheckErr(err, "SaveLog Prepare exec")
	} else {
		stmt, err := dbc.Prepare(`INSERT INTO log(datelog, host, application, message, logfile) VALUES(?, ?, ?, ?, ?)`)
		u.CheckErr(err, "SaveLog Prepare datelog")
		_, err = stmt.Exec(datelog, host, application, message, logfile)
		u.CheckErr(err, "SaveLog Prepare datelog exec")
	}
	fmt.Fprint(w, "OK log saved")
}
//HandleRequests -
func HandleRequests() {
	router := mux.NewRouter()
	router.Handle("/", isAuthorized(homePage)).Methods("GET")
	if Config.SharedToken == "" {
		log.Printf("[WARN] - SharedToken is not set. Log server will allow anyone to put log in\n")
	} else {
		router.Handle("/savelog", isAuthorized(SaveLog)).Methods("POST")
		router.Handle("/sql", isAuthorized(RunSQL)).Methods("POST")
		router.Handle("/dsl", isAuthorized(RunDSL)).Methods("POST")
		router.Handle("/backupdb", isAuthorized(BackupAndDropDB)).Methods("GET")
		router.Handle("/downloaddb", isAuthorized(DownloadDB)).Methods("GET")
		router.Handle("/version", isAuthorized(GetVersion)).Methods("GET")

		//k8s container probe
		router.HandleFunc("/container_status", ContainerStatus).Methods("GET")
	}
	srv := &http.Server{
		Addr: fmt.Sprintf(":%s", Config.Port),
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      router, // Pass our instance of gorilla/mux in.
	}
	srv.Handler = gzhttp.GzipHandler(router)
	sslKey, sslCert := Config.SslKey, Config.SslCert
	if sslKey == "" {
		log.Printf("Start server on port %s\n", Config.Port)
		log.Fatal(srv.ListenAndServe())
	} else {
		if sslKey == "auto" {
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
			log.Printf("Start SSL/TLS server with letsencrypt enabled on port %s\n", Config.Port)
			log.Fatal(srv.ListenAndServeTLS("", ""))
		} else {
			log.Printf("Start SSL/TLS server on port %s\n", Config.Port)
			log.Fatal(srv.ListenAndServeTLS(sslCert, sslKey))
		}
	}
}

func main() {
	var yamlConfigMapFile string
	flag.StringVar(&yamlConfigMapFile, "config", "", "Config map file. Used when running manually rather than in K8S. It should read the same configmap file used by the common chart helm system and have the same effect")
	flag.Parse()
	if os.Getenv("DB_PATH") == "" && yamlConfigMapFile == "" {//Generate a defauklt minimum config yaml
		yamlData := `
SHARED_TOKEN: changeme
#SERVER_DOMAIN:
#SSL_KEY:
#SSL_CERT:
SERVER_PORT: "8080"
DB_PATH: sample-db.sqlite3`
		ioutil.WriteFile("test-webapptemplate-config.yaml", []byte(yamlData), 0600)
		log.Printf("[INFO] Sample config webapptemplate-config.yaml generated. Please edit and re-run the app using the option [-config <path to config file>] later. The server will start using the generated config")
		yamlConfigMapFile = "test-webapptemplate-config.yaml"
	}
	LoadConfig(yamlConfigMapFile)
	SetUpLogDatabase()
	RunScheduleTasks()
	HandleRequests()
}

//Simple auth
func isAuthorized(endpoint func(http.ResponseWriter, *http.Request)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokens := r.Header["X-Webserver-Template-Token"]
		if len(tokens) > 0 && tokens[0] == Config.SharedToken {
			endpoint(w, r)
		} else {
			fmt.Fprintf(w, `{"error":"isAuthorized","message":"Not Authorized"}`)
		}
	})
}

//GetDBConn -
func GetDBConn() *sql.DB {
	db, err := sql.Open("sqlite3", Config.Logdbpath)
	if err != nil {
		panic(err)
	}
	if db == nil {
		panic("db nil")
	}
	return db
}

//SetUpLogDatabase -
func SetUpLogDatabase() {
	conn := GetDBConn()
	defer conn.Close()
	sql := `
	--drop table log;
	CREATE TABLE IF NOT EXISTS log(
		ts DATETIME DEFAULT(STRFTIME('%Y-%m-%d %H:%M:%f', 'NOW')),
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		host TEXT NOT NULL,
		application TEXT,
		message TEXT NOT NULL,
		logfile TEXT);
	CREATE INDEX IF NOT EXISTS log_ts ON log(ts);

	PRAGMA main.page_size = 4096;
	PRAGMA main.cache_size=10000;
	PRAGMA main.locking_mode=EXCLUSIVE;
	PRAGMA main.synchronous=NORMAL;
	PRAGMA main.journal_mode=WAL;
	PRAGMA main.cache_size=5000;`
	log.Printf("[INFO] Set up database schema\n")
	_, err := conn.Exec(sql)
	if err != nil {
		panic(err)
	}
}

func RunScheduleTasks() {
	ctab := crontab.New() // create cron table
	// AddJob and test the errors
	if err := ctab.AddJob("1 0 1 * *", DatabaseMaintenance); err != nil {
		log.Printf("[WARN] - Can not add maintanance job - %v\n", err)
	}
}
func DatabaseMaintenance() {
	conn := GetDBConn()
	defer conn.Close()
	start, _ := u.ParseTimeRange(Config.LogRetention, "")
	_startTime := start.Format("2006-01-02 15:04:05.999")
	_, err := conn.Exec(fmt.Sprintf(
		`DELETE FROM log WHERE ts < "%s";
	`, _startTime))
	if err != nil {
		log.Printf("[ERROR] - can not delete old data - %v\n", err)
	}
}
