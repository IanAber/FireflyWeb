package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
)

/**
Defines all the available API end points
*/
func setUpWebSite() {
	router := mux.NewRouter().StrictSlash(true)
	// Register with the WebSocket which will then push a JSON payload with data to keep the displayed data up to date. No polling necessary.
	router.HandleFunc("/ws", startDataWebSocket).Methods("GET")
	// Same as /ws but includes more data. This is used for the text based status page woth everything on it.
	router.HandleFunc("/wsFull", startStatusWebSocket).Methods("GET")
	// Returns the status page with text based information on all components
	router.HandleFunc("/status", getStatus).Methods("GET")
	// Returns JSON data containing error flag information from the fuel cell
	router.HandleFunc("/fcerrors", getFuelCellErrors).Methods("GET")

	// Returns data used to create the fuel cell performance graphs
	router.HandleFunc("/fcdata/{device}/{from}/{to}", getFuelCellHistory).Methods("GET")
	// Turn off the fuel cell (device is 0 or 1)

	// Turns the fuel cell on or off (device is 0 or 1) payload is {"state":true} or {"state":false}
	router.HandleFunc("/fc/on_off", setFcOnOff).Methods("PUT")

	// Start the fuel cell (device is 0 or 1) payload is {"state":true} or {"state":false}
	router.HandleFunc("/fc/run", setFcRun).Methods("PUT")

	// Do a complete shutdown and restart of the fuel cell (device is 0 or 1)
	router.HandleFunc("/fc/{device}/restart", fcRestart).Methods("PUT")
	// Returns the fuel cell status (device is 0 or 1)
	router.HandleFunc("/fc/{device}/status", fcStatus).Methods("GET")
	// Turns maintenance mode on or off to allow for reprogramming the fuel cells. Uses the same payload as on_off above

	router.HandleFunc("/fc/maintenance", setFcMaintenance).Methods("PUT")

	// Turn the gas to the fuel cell on or off payloa = {"state":true} or {"state":false}
	router.HandleFunc("/gas", setGas).Methods("PUT")

	// Turn the spare relay on or off payloa = {"state":true} or {"state":false}
	router.HandleFunc("/spare", setSpare).Methods("PUT")

	router.HandleFunc("/el/{device}", elCommand).Methods("PUT")

	router.HandleFunc("/eldetail/{device}/{from}/{to}", getElectrolyserDetail).Methods("GET")
	router.HandleFunc("/el/{device}/on", setElOn).Methods("GET", "POST")
	router.HandleFunc("/el/{device}/off", setElOff).Methods("GET", "POST")
	router.HandleFunc("/el/{device}/setRate", showElectrolyserProductionRatePage).Methods("GET", "POST")
	router.HandleFunc("/el/{device}/status", getElectrolyserJsonStatus).Methods("GET")
	router.HandleFunc("/el/{device}/start", startElectrolyser).Methods("GET", "POST")
	router.HandleFunc("/el/{device}/stop", stopElectrolyser).Methods("GET", "POST")
	router.HandleFunc("/el/{device}/reboot", rebootElectrolyser).Methods("GET", "POST")
	router.HandleFunc("/el/{device}/preheat", preheatElectrolyser).Methods("GET", "POST")
	router.HandleFunc("/el/{device}/restartPressure/{bar}", setRestartPressure).Methods("GET", "POST")
	router.HandleFunc("/el/start", startAllElectrolysers).Methods("GET", "POST")
	router.HandleFunc("/el/stop", stopAllElectrolysers).Methods("GET", "POST")
	router.HandleFunc("/el/reboot", rebootAllElectrolysers).Methods("POST")
	router.HandleFunc("/el/preheat", preheatAllElectrolysers).Methods("GET", "POST")
	router.HandleFunc("/el/setrate", setElectrolyserRate).Methods("POST")
	router.HandleFunc("/el/getRate", getElectrolyserRate).Methods("GET")
	router.HandleFunc("/el/on", setAllElOn).Methods("POST")
	router.HandleFunc("/el/off", setAllElOff).Methods("POST")
	router.HandleFunc("/miscdata/{from}/{to}", getHistory).Methods("GET")
	router.HandleFunc("/dr/{device}/status", getDryerJsonStatus).Methods("GET")
	router.HandleFunc("/dr/reboot", rebootDryer).Methods("POST")
	router.HandleFunc("/minStatus", getMinHtmlStatus).Methods("GET")
	router.HandleFunc("/eldata/{from}/{to}", getElectrolyserHistory).Methods("GET")
	router.HandleFunc("/powerdata/{from}/{to}", getPowerData).Methods("GET")
	router.HandleFunc("/co2saved", getCO2Saved).Methods("GET")
	router.HandleFunc("/system", getSystem).Methods("GET")
	router.HandleFunc("/candump/{from}/{to}", candump).Methods("GET")
	router.HandleFunc("/candumpEvent/{event}", candumpEvent).Methods("GET")
	router.HandleFunc("/canrecord/{to}", canRecord).Methods("GET")
	router.HandleFunc("/canEvents", listCANEvents).Methods("GET")
	router.HandleFunc("/settings", getSettings).Methods("GET")
	router.HandleFunc("/settings", updateSettings).Methods("POST")
	fileServer := http.FileServer(neuteredFileSystem{http.Dir("/Firefly/web")})
	router.PathPrefix("/").Handler(http.StripPrefix("/", fileServer))

	log.Println("Starting WEB server")
	log.Fatal(http.ListenAndServe(":20080", router))
}

/**
setFcRun Starts or Stops the fuel cell
 Turns on the fuel cel and gas if needed during start
*/
func setFcRun(w http.ResponseWriter, r *http.Request) {
	var jBody OnOffPayload
	jBody.Device = 0xff // Set the device to 0xFF so unless it is supplied in the payload it will be invalid

	// Get the payload body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		ReturnJSONError(w, "Fuel Cell", err, http.StatusBadRequest, true)
		return
	}
	err = json.Unmarshal(body, &jBody)
	if err != nil {
		ReturnJSONError(w, "Fuel Cell", err, http.StatusBadRequest, true)
		return
	}

	log.Println(jBody)

	// Device should be 0 or 1.
	if (err != nil) || (jBody.Device > 1) {
		ReturnJSONErrorString(w, "Fuel Cell", "Invalid fuel cell in 'run' request", http.StatusBadRequest, true)
		return
	}

	if jBody.State {
		// Start the cell
		err = startFuelCell(jBody.Device)
	} else {
		// Stop the cell
		err = stopFuelCell(jBody.Device)
	}
	if err != nil {
		ReturnJSONError(w, "Fuel Cell", err, http.StatusInternalServerError, true)
		return
	}
	returnJSONSuccess(w)
}

/**
setFcOnOff enables or disables the fuel cell
*/
func setFcOnOff(w http.ResponseWriter, r *http.Request) {
	var jBody OnOffPayload
	jBody.Device = 0xff // Set the device to 0xFF so unless it is supplied in the payload it will be invalid

	log.Println("FuelCell On/Off")
	// Get the payload body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		ReturnJSONError(w, "Fuel Cell", err, http.StatusBadRequest, true)
		return
	}
	err = json.Unmarshal(body, &jBody)
	if err != nil {
		ReturnJSONError(w, "Fuel Cell", err, http.StatusBadRequest, true)
		return
	}

	// Device should be 0 or 1.
	if (err != nil) || (jBody.Device > 1) {
		ReturnJSONErrorString(w, "Fuel Cell", "Invalid fuel cell in 'on/off' request", http.StatusBadRequest, true)
		return
	}

	if jBody.State {
		// Start the cell
		log.Print("Turn on the fuel cell")
		err = turnOnFuelCell(jBody.Device)
	} else {
		// Stop the cell
		log.Print("Turn off the fuel cell")
		err = turnOffFuelCell(jBody.Device)
	}
	if err != nil {
		ReturnJSONError(w, "Fuel Cell", err, http.StatusInternalServerError, true)
		return
	}
	returnJSONSuccess(w)
}

func fcRestart(w http.ResponseWriter, r *http.Request) {
	var jErr JSONError

	vars := mux.Vars(r)
	device, err := parseDevice(vars["device"])
	if (err != nil) || (device > 1) || (device < 0) {
		log.Println(jErr.AddErrorString("Fuel Cell", "Invalid fuel cell in 'status' request"))
		jErr.ReturnError(w, 400)
		return
	}

	// Device should be 0 or 1.
	if (err != nil) || (device > 1) {
		ReturnJSONErrorString(w, "Fuel Cell", "Invalid fuel cell in 'on/off' request", http.StatusBadRequest, true)
		return
	}

	if err = restartFc(device); err != nil {
		ReturnJSONError(w, "Fuel Cell", err, http.StatusInternalServerError, true)
		return
	}
	returnJSONSuccess(w)
}

func setFcMaintenance(w http.ResponseWriter, r *http.Request) {
	var jStatus struct {
		On bool `json:"maintenance"`
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		ReturnJSONError(w, "Fuel Cell", err, http.StatusInternalServerError, true)
		return
	}
	err = json.Unmarshal(body, &jStatus)
	if err != nil {
		ReturnJSONError(w, "Fuel Cell", err, http.StatusInternalServerError, true)
	}
	params.FuelCellMaintenance = jStatus.On
	if err := params.WriteSettings(); err != nil {
		log.Print(err)
	}
	returnJSONSuccess(w)
}
