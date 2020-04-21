package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"time"

	"github.com/godbus/dbus"
	"github.com/gorilla/mux"

	camera "github.com/TheCacophonyProject/fake-thermal-camera/fakecamera"
	config "github.com/TheCacophonyProject/go-config"
	arg "github.com/alexflint/go-arg"
)

type argSpec struct {
	CPTVDir   string `arg:"-c,--cptv-dir" help:"base path of cptv files"`
	ConfigDir string `arg:"-c,--config" help:"path to configuration directory"`
}

var (
	cptvDir = "/cptv-files"
)

func procArgs() argSpec {
	args := argSpec{CPTVDir: cptvDir}
	args.ConfigDir = config.DefaultConfigDir

	arg.MustParse(&args)
	return args
}

func main() {
	args := procArgs()
	go camera.RunCamera(args.CPTVDir, args.ConfigDir)
	if err := runServer(); err != nil {
		log.Fatal(err)
	}

	if err := runServer(); err != nil {
		log.Fatal(err)
	}
}

var version = "<not set>"

func runServer() error {
	log.SetFlags(0)

	router := mux.NewRouter()
	router.HandleFunc("/create/{device-name}", createDeviceHandler)
	router.HandleFunc("/", homeHandler)
	router.HandleFunc("/triggerEvent/{type}", triggerEventHandler)
	router.HandleFunc("/sendCPTVFrames", sendCPTVFramesHandler)
	router.HandleFunc("/clearCPTVQueue", clearCPTVQueue)

	log.Fatal(http.ListenAndServe(":2040", router))
	return nil
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "This is a Fake thermal camera test server.")
}

func createDeviceHandler(w http.ResponseWriter, r *http.Request) {
	deviceName := mux.Vars(r)["device-name"]
	groupnames, ok := r.URL.Query()["group-name"]
	if !ok {
		logError("'group-name' query parameter is missing", w, http.StatusBadRequest)
	} else {
		apiServers, ok := r.URL.Query()["api-server"]
		apiServer := "https://api-test.cacophony.org.nz"
		if ok {
			apiServer = apiServers[0]
		}
		log.Printf("Creating device " + deviceName + " and group-name " + groupnames[0] + " on server " + apiServer)
		cmd := exec.Command("./device-register",
			"--name",
			deviceName,
			"--group",
			groupnames[0],
			"--password",
			"password_"+deviceName,
			"--api",
			apiServer,
			"--ignore-minion-id",
			"--remove-device-config")

		cmd.Dir = "/code/device-register"

		if output, err := cmd.CombinedOutput(); err != nil {
			outputString := string(output)
			logError(fmt.Sprintf("Error registering device %v", outputString), w, http.StatusInternalServerError)
		} else {
			log.Printf("device created")
			restartThermalUploader()
			deviceID, err := getDeviceID()
			if err != nil {
				logError(fmt.Sprintf("Could not read device id %v", err), w, http.StatusInternalServerError)
				return
			}
			io.WriteString(w, fmt.Sprintf("%d", deviceID))
		}
	}
}

func restartThermalUploader() {
	log.Printf("restarting thermal uploader")
	cmd := exec.Command("supervisorctl", "restart", "thermal-uploader")
	cmd.Start()
}
func getDeviceID() (int, error) {
	configRW, err := config.New(config.DefaultConfigDir)
	if err != nil {
		return 0, err
	}
	var deviceConfig config.Device
	if err := configRW.Unmarshal(config.DeviceKey, &deviceConfig); err != nil {
		return 0, err
	}
	return deviceConfig.ID, nil
}

func triggerEventHandler(w http.ResponseWriter, r *http.Request) {
	eventType := mux.Vars(r)["type"]
	eventDetails := map[string]interface{}{
		"description": map[string]interface{}{
			"type": eventType,
		},
	}
	ts := time.Now()
	detailsJSON, err := json.Marshal(&eventDetails)
	if err != nil {
		logError(fmt.Sprintf("Could not marshal json %s: %s", eventDetails, err), w, http.StatusInternalServerError)
		return
	}

	conn, err := dbus.SystemBus()
	if err != nil {
		logError(fmt.Sprintf("Could not connect to dbus: %s", err), w, http.StatusInternalServerError)
		return
	}

	obj := conn.Object("org.cacophony.Events", "/org/cacophony/Events")
	call := obj.Call("org.cacophony.Events.Add", 0, string(detailsJSON), eventType, ts.UnixNano())
	if call.Err != nil {
		logError(fmt.Sprintf("Could not record %s event: %s", eventType, call.Err), w, http.StatusInternalServerError)
		return
	}
}

func sendCPTVFramesHandler(w http.ResponseWriter, r *http.Request) {
	queryVars := r.URL.Query()
	fileName := queryVars.Get("cptv-file")
	if fileName == "" {
		queryVars.Set("cptv-file", "person.cptv")
	}
	log.Printf("")
	camera.Send(queryVars)

	log.Printf("Sent CPTV Frames")
	io.WriteString(w, "Success")
}

func clearCPTVQueue(w http.ResponseWriter, r *http.Request) {
	stop, _ := strconv.ParseBool(r.URL.Query().Get("stop"))
	camera.ClearQueue(stop)

	log.Printf("Clear CPTV Frames")
	io.WriteString(w, "Success")
}

func logError(errorString string, w http.ResponseWriter, code int) {
	log.Printf("Error: %s", errorString)
	http.Error(w, fmt.Sprintf(errorString), code)
}
