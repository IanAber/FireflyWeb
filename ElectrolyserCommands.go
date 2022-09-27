package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Rate struct {
	el0      uint8
	el1      uint8
	elSingle uint8
}

var RateArray = []Rate{{0, 0, 0},
	{60, 0, 60},
	{61, 0, 60},
	{63, 0, 61},
	{64, 0, 61},
	{65, 0, 62},
	{66, 0, 62},
	{67, 0, 62},
	{69, 0, 63},
	{70, 0, 63},
	{71, 0, 64},
	{72, 0, 64},
	{74, 0, 64},
	{75, 0, 65},
	{76, 0, 65},
	{77, 0, 66},
	{78, 0, 66},
	{80, 0, 66},
	{81, 0, 67},
	{82, 0, 67},
	{83, 0, 68},
	{85, 0, 68},
	{86, 0, 68},
	{87, 0, 69},
	{88, 0, 69},
	{89, 0, 70},
	{91, 0, 70},
	{92, 0, 70},
	{93, 0, 71},
	{94, 0, 71},
	{95, 0, 72},
	{97, 0, 72},
	{98, 0, 72},
	{99, 0, 73},
	{100, 0, 73},
	{60, 61, 74},
	{60, 62, 74},
	{60, 63, 74},
	{60, 64, 75},
	{60, 65, 75},
	{60, 67, 76},
	{60, 68, 76},
	{60, 69, 76},
	{60, 70, 77},
	{60, 72, 77},
	{60, 73, 78},
	{60, 74, 78},
	{60, 75, 78},
	{60, 76, 79},
	{60, 78, 79},
	{60, 79, 80},
	{60, 80, 80},
	{60, 81, 80},
	{60, 83, 81},
	{60, 84, 81},
	{60, 85, 82},
	{60, 86, 82},
	{60, 87, 82},
	{60, 89, 83},
	{60, 90, 83},
	{60, 91, 84},
	{60, 92, 84},
	{60, 94, 84},
	{60, 95, 85},
	{60, 96, 85},
	{60, 97, 86},
	{60, 98, 86},
	{60, 100, 86},
	{61, 100, 87},
	{62, 100, 87},
	{63, 100, 88},
	{65, 100, 88},
	{66, 100, 88},
	{67, 100, 89},
	{68, 100, 89},
	{69, 100, 90},
	{71, 100, 90},
	{72, 100, 90},
	{73, 100, 91},
	{74, 100, 91},
	{75, 100, 92},
	{77, 100, 92},
	{78, 100, 92},
	{79, 100, 93},
	{81, 100, 93},
	{82, 100, 94},
	{83, 100, 94},
	{84, 100, 94},
	{85, 100, 95},
	{86, 100, 95},
	{88, 100, 96},
	{89, 100, 96},
	{90, 100, 96},
	{91, 100, 97},
	{93, 100, 97},
	{94, 100, 98},
	{95, 100, 98},
	{96, 100, 98},
	{97, 100, 99},
	{99, 100, 99},
	{100, 100, 100}}

var CurrentRate uint8

func init() {
	CurrentRate = 0
}

/*
elCommand handles the On, Off, Start and Stop web commands to each electrolyser
*/
func elCommand(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Command string `json:"command"`
		device  int
	}

	vars := mux.Vars(r)
	deviceStr := vars["device"]

	switch deviceStr {
	case "0":
		body.device = 0
	case "1":
		body.device = 1
	default:
		ReturnJSONErrorString(w, "Electrolyser", "Invalid device - "+deviceStr, http.StatusBadRequest, true)
		return
	}

	if bytes, err := io.ReadAll(r.Body); err != nil {
		ReturnJSONError(w, "Electrolyser", err, http.StatusInternalServerError, true)
		return
	} else {
		debugPrint(string(bytes))
		if err := json.Unmarshal(bytes, &body); err != nil {
			ReturnJSONError(w, "Electrolyser", err, http.StatusBadRequest, true)
			return
		}
	}
	body.Command = strings.ToLower(body.Command)

	var err error
	switch body.Command {
	case "on":

		if body.device == 0 {
			err = mbusRTU.EL0OnOff(true)
		} else {
			err = mbusRTU.EL1OnOff(true)
		}
		if err != nil {
			ReturnJSONError(w, "Electrolyser", err, http.StatusInternalServerError, true)
		}
	case "off":
		if body.device == 0 {
			if len(SystemStatus.Electrolysers) > 0 {
				if SystemStatus.Electrolysers[0].status.StackVoltage > jsonFloat32(params.ElectrolyserMaxStackVoltsTurnOff) {
					ReturnJSONErrorString(w, "Electrolyser", "Electrolyser 0 not turned off because stack voltage is too high.", http.StatusBadRequest, true)
					return
				}
			}
			err = mbusRTU.EL0OnOff(false)
		} else {
			if len(SystemStatus.Electrolysers) > 1 {
				if SystemStatus.Electrolysers[1].status.StackVoltage > jsonFloat32(params.ElectrolyserMaxStackVoltsTurnOff) {
					ReturnJSONErrorString(w, "Electrolyser", "Electrolyser 1 not turned off because stack voltage is too high.", http.StatusBadRequest, true)
					return
				}
			}
			err = mbusRTU.EL1OnOff(false)
		}
		if err != nil {
			ReturnJSONError(w, "Electrolyser", err, http.StatusInternalServerError, true)
			return
		}
	case "start":
		if SystemStatus.Electrolysers[body.device].IsSwitchedOn() {
			if !SystemStatus.Electrolysers[body.device].Start(true) {
				ReturnJSONErrorString(w, "Electrolyser", "Failed to start the electrolyser", http.StatusInternalServerError, true)
				return
			}
		} else {
			ReturnJSONErrorString(w, "Electrolyser", "Failed to start the electrolyser - it is not powered on", http.StatusBadRequest, true)
			return
		}
	case "stop":
		if SystemStatus.Electrolysers[body.device].IsSwitchedOn() {
			if !SystemStatus.Electrolysers[body.device].Stop(true) {
				ReturnJSONErrorString(w, "Electrolyser", "Failed to stop the electrolyser", http.StatusInternalServerError, true)
				return
			}
		} else {
			ReturnJSONErrorString(w, "Electrolyser", "Failed to stop the electrolyser - it is not powered on", http.StatusBadRequest, true)
			return
		}
	default:
		ReturnJSONErrorString(w, "Electrolyser", "Unknown command -"+body.Command, http.StatusBadRequest, true)
		return
	}
	returnJSONSuccess(w)
}

/*
setElectrolyserPercentRate sets the given electrolyser to the given rate
*/
func setElectrolyserPercentRate(rate uint8, device uint8) error {
	if uint8(len(SystemStatus.Electrolysers)) <= device {
		return fmt.Errorf("Invalid electrolyser")
	}
	if SystemStatus.Electrolysers[device].status.SwitchedOn {
		if rate > 0 {
			if SystemStatus.Electrolysers[device].status.ElState == ElIdle && (rate > 0) {
				// State is idle so start it first if not in holdoff
				if time.Now().After(SystemStatus.Electrolysers[device].OnOffTime.Add(ELECTROLYSERHOLDOFFTIME)) {
					log.Println("Start electrolyser ", device)
					SystemStatus.Electrolysers[device].Start(false)
					SystemStatus.Electrolysers[device].OnOffTime = time.Now()
				} else {
					log.Println("Electrolyser ", device, " is in hold off so is not starting. Waiting until ", SystemStatus.Electrolysers[device].OnOffTime.Add(ELECTROLYSERHOLDOFFTIME).Format("15:04:05"))
				}
			}
		}
		SystemStatus.Electrolysers[device].SetProduction(rate)
	} else {
		// Not switched on so if we are setting to more than 0 fire it up as long as we are below the restart pressure
		if SystemStatus.Gas.TankPressure < ELECTROLYSERRESTARTPRESSURE && rate > 0 {
			var err error
			if device == 0 {
				err = mbusRTU.EL0OnOff(true)
			} else {
				err = mbusRTU.EL1OnOff(true)
			}
			if err != nil {
				log.Print(err)
			}
		}
	}
	return nil
}

/*
preheatElectrolyser tells the given electrolyser to preheat the electrolyte
*/
func preheatElectrolyser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device := vars["device"]
	deviceNum, err := strconv.ParseInt(device, 10, 8)
	if err != nil {
		log.Println("Failed to get the device. - ", err)
	}
	if err := validateDevice(uint8(deviceNum)); err != nil {
		log.Println("Invalid device requeted in preheat - ", err)
	}

	SystemStatus.Electrolysers[deviceNum].Preheat()
	if _, err := fmt.Fprintf(w, "Electrolyser %d preheat requested", deviceNum); err != nil {
		log.Println("Error returning status after electrolyser preheat request. - ", err)
	}
}

/*
preheatAllElectrolysers tells all electrolysers to preheat the electrolyte
*/
func preheatAllElectrolysers(w http.ResponseWriter, _ *http.Request) {
	for _, el := range SystemStatus.Electrolysers {
		el.Preheat()
	}
	if _, err := fmt.Fprintf(w, "Electrolyser preheat requested"); err != nil {
		log.Println("Error returning status after electrolyser preheat all request. - ", err)
	}
}

/*
startElectrolyser starts the given electrolyser
*/
func startElectrolyser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device := vars["device"]
	deviceNum, err := strconv.ParseInt(device, 10, 8)
	if err != nil {
		log.Println("Failed to get the device. - ", err)
	}
	if err := validateDevice(uint8(deviceNum)); err != nil {
		log.Println("Invalid device requeted in selElOn - ", err)
	}

	// Start immediately
	SystemStatus.Electrolysers[deviceNum].Start(true)
	if _, err := fmt.Fprintf(w, "Electrolyser start requested"); err != nil {
		log.Println("Error returning status after electrolyser start request. - ", err)
	}
}

/*
startAllElectrolysers starts all electrolysers
*/
func startAllElectrolysers(w http.ResponseWriter, _ *http.Request) {
	for _, el := range SystemStatus.Electrolysers {
		// Start all immediately
		el.Start(true)
	}
	if _, err := fmt.Fprintf(w, "Electrolyser start requested"); err != nil {
		log.Println("Error returning status after electrolyser start request. - ", err)
	}
}

/*
stopElectrolyser stops the given electrolyser immediately
*/
func stopElectrolyser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device := vars["device"]
	deviceNum, err := strconv.ParseInt(device, 10, 8)
	if err != nil {
		ReturnJSONError(w, "Electrolyser", err, http.StatusBadRequest, true)
		return
	}
	if err := validateDevice(uint8(deviceNum)); err != nil {
		ReturnJSONError(w, "Electrolyser", err, http.StatusBadRequest, true)
		return
	}

	SystemStatus.Electrolysers[deviceNum].Stop(true)
	returnJSONSuccess(w)
}

/*
stopAllElectrolysers stops all electrolysers
*/
func stopAllElectrolysers(w http.ResponseWriter, _ *http.Request) {
	for _, el := range SystemStatus.Electrolysers {
		// Immediate shut down
		el.Stop(true)
	}
	if _, err := fmt.Fprintf(w, "Electrolyser stop requested"); err != nil {
		log.Println("Error returning status after electrolyser stop request. - ", err)
	}
}

/*
rebootElectrolyser reboots the given electrolyser
*/
func rebootElectrolyser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device := vars["device"]
	deviceNum, err := strconv.ParseInt(device, 10, 8)
	if err != nil {
		log.Println("Failed to get the device. - ", err)
	}
	if err := validateDevice(uint8(deviceNum)); err != nil {
		log.Println("Invalid device requeted in selElOn - ", err)
	}
	SystemStatus.Electrolysers[deviceNum].Reboot()
	returnJSONSuccess(w)
}

/*
rebootAllElectrolysers sends a reboot command to all electrolysers
*/
func rebootAllElectrolysers(w http.ResponseWriter, _ *http.Request) {
	for _, el := range SystemStatus.Electrolysers {
		el.Reboot()
	}
	returnJSONSuccess(w)
}

/*
setProductionRates sets all electrolysers to the selected rate.
*/
func setProductionRates(rate uint8) error {
	CurrentRate = rate
	var elRates Rate
	if len(SystemStatus.Electrolysers) > 1 {
		elRates = RateArray[rate]
	} else {
		elRates.el0 = 0
		elRates.el1 = 0
		if rate == 0 {
			elRates.el0 = 0
		} else {
			elRates.el0 = uint8((rate*4)/10) + 60
		}
	}
	if err := setElectrolyserPercentRate(elRates.el0, 0); err != nil {
		return err
	}
	if len(SystemStatus.Electrolysers) > 1 {
		if err := setElectrolyserPercentRate(elRates.el1, 1); err != nil {
			return err
		}
	}
	return nil
}

/**
set the electrolyser selected rate.
*/
func setElectrolyserRate(w http.ResponseWriter, r *http.Request) {
	var responseJson struct {
		EL0     int16    `json:"el0"`
		EL1     int16    `json:"el1"`
		Success bool     `json:"success"`
		Errors  []string `json:"errors"`
	}

	responseJson.Success = true
	body, err := io.ReadAll(r.Body)
	if err != nil {
		ReturnJSONError(w, "Electrolyser", err, http.StatusInternalServerError, true)
		return
	}
	var jRate struct {
		Rate int64 `json:"rate"`
	}
	if err = json.Unmarshal(body, &jRate); err != nil {
		log.Println(err)
		return
	}
	debugPrint("Set electrolyser : %d", jRate.Rate)

	_, err = pDB.Exec("INSERT INTO ElectrolyserRequests (RateRequested) VALUES (?)", jRate.Rate)
	if err != nil {
		log.Println("Log Electrolyser Request - ", err)
	}

	// Return bad request if outside acceptable range of 0..100%
	if jRate.Rate > 100 || jRate.Rate < 0 {
		ReturnJSONErrorString(w, "Electrolyser", "Rate must be between 0 and 100", http.StatusBadRequest, true)
		return
	}

	if (SystemStatus.Relays.FC0Run || SystemStatus.Relays.FC1Run) && jRate.Rate > 0 {
		// Do not allow the electrolysers to run if one or more fuel cells are also running
		jRate.Rate = 0

		for _, el := range SystemStatus.Electrolysers {
			// Immediate shut down
			el.Stop(true)
			CurrentRate = 0
		}
		ReturnJSONErrorString(w, "Electrolyser", "One or more Fuel cells are running. All electrolysers are stopped.", http.StatusBadRequest, false)
		return
	}

	if err := setProductionRates(uint8(jRate.Rate)); err != nil {
		ReturnJSONError(w, "Electrolyser", err, http.StatusInternalServerError, true)
		return
	}
	returnJSONSuccess(w)
}

/**
Return the total electrolyser rate as a percentage, 0-100%
*/
func getElectrolyserRate(w http.ResponseWriter, _ *http.Request) {

	var jReturnData struct {
		Rate   uint8   `json:"rate"`
		Gas    float64 `json:"gas"`
		Status string  `json:"status"`
	}

	// Set the gas pressure
	jReturnData.Gas = SystemStatus.Gas.TankPressure
	jReturnData.Rate = CurrentRate

	// Perhaps we should ensure that the electrolysers are where we are saying they are.
	//	debugPrint("Forcing rates in GetRate command")
	if err := setProductionRates(CurrentRate); err != nil {
		log.Println(err)
	}

	// Loop through and find if any of the electrolysers are on
	ElectrolysersSwitchedOn := false
	for _, e := range SystemStatus.Electrolysers {
		if e.IsSwitchedOn() {
			// If anyone is switched off then assume all are off
			ElectrolysersSwitchedOn = true
		}
	}
	if ElectrolysersSwitchedOn {
		// If any electrolysers are on get the production settings
		if len(SystemStatus.Electrolysers) == 1 {
			// We only have one electrolyser so this is easy
			switch SystemStatus.Electrolysers[0].status.ElState {
			case ElIdle:
				jReturnData.Status = "Idle"
			case ElStandby:
				jReturnData.Status = "Standby"
			default:
				jReturnData.Status = "Active"
			}
		} else {
			// We have two electrolysers, so we need to figure out what to send as the status
			switch SystemStatus.Electrolysers[0].status.ElState {
			case ElIdle:
				jReturnData.Status = "Idle"
			case ElStandby:
				jReturnData.Status = "Standby"
			default:
				jReturnData.Status = "Active"
			}

			switch SystemStatus.Electrolysers[1].status.ElState {
			case ElIdle:
				if jReturnData.Status != "Standby" {
					jReturnData.Status = "Idle"
				}
			case ElStandby:
				jReturnData.Status = "Standby"
			default:
				if jReturnData.Status != "Standby" {
					jReturnData.Status = "Idle"
				}
				jReturnData.Status = "Active"
			}
		}
	} else {
		// all electrolysers are off, so we report as OFF.
		jReturnData.Rate = 0
		CurrentRate = 0
		jReturnData.Status = "OFF"
	}

	if bytesArray, err := json.Marshal(jReturnData); err != nil {
		ReturnJSONError(w, "Electrolyser", err, http.StatusInternalServerError, true)
	} else {
		if _, err := fmt.Fprint(w, string(bytesArray)); err != nil {
			log.Print(err)
		}
	}
}

/**
setRestartPressure allows the system to program the electrolyser maximum restart pressur below which it will
automatically start producing hydrogen.
URL = /el/{device}/restartPressure/{bar}
*/
func setRestartPressure(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sDevice := vars["device"]
	device, err := strconv.ParseInt(sDevice, 10, 8)
	if err != nil {
		ReturnJSONErrorString(w, "Electrolyser", "Invalid device specified - device was not numeric", http.StatusBadRequest, true)
		return
	}
	sPressure := vars["bar"]
	pressure, err := strconv.ParseFloat(sPressure, 32)
	if err != nil {
		ReturnJSONErrorString(w, "Electrolyser", "Invalid pressure specified - pressure was not numeric", http.StatusBadRequest, true)
		return
	}

	if err := validateDevice(uint8(device)); err != nil {
		log.Println("Invalid device requeted in setRestartPressure - ", err)
	}
	if (pressure < 2.0) || (pressure > 35.0) {
		ReturnJSONErrorString(w, "Electrolyser", "Invalid pressure specified (2..35)", http.StatusBadRequest, true)
		return
	}

	err = SystemStatus.Electrolysers[device].SetRestartPressure(float32(pressure))
	if err != nil {
		ReturnJSONError(w, "Electrolyser", err, http.StatusInternalServerError, true)
		return
	}
	returnJSONSuccess(w)
}

func getElectrolyserDetail(w http.ResponseWriter, r *http.Request) {
	type Row struct {
		Logged           string  `json:"logged"`
		Rate             float32 `json:"rate"`
		ElectrolyteLevel string  `json:"electrolyteLevel"`
		ElectrolyteTemp  float32 `json:"electrolyteTemp"`
		Flow             float32 `json:"flow"`
		H2InnerPressure  float32 `json:"h2InnerPressure"`
		H2OuterPressure  float32 `json:"h2OuterPressure"`
		StackVoltage     float32 `json:"stackVoltage"`
		//		SystemState      string  `json:"systemState"`
		WaterPressure float32 `json:"waterPressure"`
		StackCurrent  float32 `json:"stackCurrent"`
	}
	var results []*Row
	var el int32

	// Set the returned type to application/json
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	from := vars["from"]
	to := vars["to"]
	device := vars["device"]
	switch device {
	case "1":
		el = 0
	case "2":
		el = 1
	default:
		err := fmt.Errorf("invalid device - %s", device)
		ReturnJSONError(w, "Electrolyser", err, http.StatusBadRequest, true)
		return
	}
	var sSQL string
	if el == 0 {
		sSQL = `SELECT MIN(UNIX_TIMESTAMP(logged)) AS logged, AVG(el0Rate) AS rate, LAST_VALUE(el0ElectrolyteLevel) AS electrolyteLevel, AVG(el0ElectrolyteTemp) AS electrolyteTemp, AVG(el0H2Flow) AS flow, 
AVG(el0H2InnerPressure) AS h2InnerPressure, AVG(el0H2OuterPressure) AS h2OuterPressure, AVG(el0StackVoltage) AS stackVoltage, AVG(el0WaterPressure) AS waterPressure, AVG(el0StackCurrent) AS stackCurrent
FROM firefly.logging
WHERE el0Rate is not null AND logged BETWEEN ? AND ?
GROUP BY UNIX_TIMESTAMP(logged) DIV ?;`
	} else {
		sSQL = `SELECT MIN(UNIX_TIMESTAMP(logged)) AS logged, AVG(el1Rate) AS rate, LAST_VALUE(el1ElectrolyteLevel) AS electrolyteLevel, AVG(el1ElectrolyteTemp) AS electrolyteTemp, AVG(el1H2Flow) AS flow, 
AVG(el1H2InnerPressure) AS h2InnerPressure, AVG(el1H2OuterPressure) AS h2OuterPressure, AVG(el1StackVoltage) AS stackVoltage, AVG(el1WaterPressure) AS waterPressure, AVG(el1StackCurrent) AS stackCurrent
FROM firefly.logging
WHERE el1Rate is not null AND logged BETWEEN ? AND ?
GROUP BY UNIX_TIMESTAMP(logged) DIV ?;`
	}

	fromTime, err := time.ParseInLocation("2006-1-2T15:4", from, time.Local)
	if err != nil {
		ReturnJSONError(w, "Electrolyser", err, http.StatusBadRequest, true)
		return
	}
	toTime, err := time.ParseInLocation("2006-1-2T15:4", to, time.Local)
	if err != nil {
		ReturnJSONError(w, "Electrolyser", err, http.StatusBadRequest, true)
		return
	}

	// Get the difference in the two times in nanoseconds
	timeDiff := toTime.Sub(fromTime)
	// We want around 300 points so find out how many seconds per point that would be
	frequency := timeDiff / 300
	rows, err := pDB.Query(sSQL, from, to, frequency.Seconds())
	if err != nil {
		ReturnJSONError(w, "Electrolyser", err, http.StatusInternalServerError, true)
		return
	}
	log.Println("Frequency =", frequency.Seconds())

	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err)
		}
	}()
	for rows.Next() {
		row := new(Row)
		if err := rows.Scan(&(row.Logged), &(row.Rate), &(row.ElectrolyteLevel), &(row.ElectrolyteTemp), &(row.Flow),
			&(row.H2InnerPressure), &(row.H2OuterPressure), &(row.StackVoltage), &(row.WaterPressure), &(row.StackCurrent)); err != nil {
			ReturnJSONError(w, "Electrolyser", err, http.StatusInternalServerError, true)
			return
		} else {
			results = append(results, row)
		}
	}
	if len(results) == 0 {
		ReturnJSONErrorString(w, "Electrolyser", "No results found - "+from+" | "+to+" | device = "+device+` | `+sSQL, http.StatusBadRequest, true)
		return
	}
	if JSON, err := json.Marshal(results); err != nil {
		ReturnJSONError(w, "Electrolyser", err, http.StatusInternalServerError, true)
	} else {
		if _, err := fmt.Fprintf(w, string(JSON)); err != nil {
			log.Println(err)
		}
	}
}

/**
validateDevice ensures that the device represents a known electrolyser. Device is 0 based
*/
func validateDevice(device uint8) error {
	if device >= uint8(len(SystemStatus.Electrolysers)) {
		return fmt.Errorf("invalid Electrolyser device - %d", device)
	}
	return nil
}

func getElectrolyserHtmlStatus(El *Electrolyser) (html string) {
	// Check the relay status to ensure power is being provided to the electrolyser
	if (El.status.Device == 0 && !SystemStatus.Relays.EL0) || (El.status.Device == 1 && !SystemStatus.Relays.EL1) {
		html = `<h3 style="text-align:center">Electrolyser is switched OFF</h3`
		return
	}

	html = fmt.Sprintf(`<table>
  <tr><td class="label">System State</td><td>%s</td><td class="label">Electrolyser State</td><td>%s</td></tr>
  <tr><td class="label">Electrolyte Level</td><td>%s</td><td class="label">Electrolyte Temp</td><td>%0.1f ℃</td></tr>
  <tr><td class="label">Inner H2 Pressure</td><td>%0.2f bar</td><td class="label">Outer H2 Pressure</td><td>%0.2f bar</td></tr>
  <tr><td class="label">H2 Flow</td><td>%0.2f NL/hour</td><td class="label">Water Pressure</td><td>%0.1f bar</td></tr>
  <tr><td class="label">Max Tank Pressure</td><td>%0.1f bar</td><td class="label">Restart Pressure</td><td>%0.1f bar</td></tr>
  <tr><td class="label">Current Production Rate</td><td>%d%%</td><td class="label">Default Production Rate</td><td>%0.1f%%</td></tr>
  <tr><td class="label">Stack Voltage</td><td>%0.2f volts</td><td class="label">Serial Number</td><td>%s</td></tr>
</table>`, El.GetSystemState(), El.getState(), El.status.ElectrolyteLevel.String(), El.status.ElectrolyteTemp,
		El.status.InnerH2Pressure, El.status.OuterH2Pressure, El.status.H2Flow, El.status.WaterPressure,
		El.status.MaxTankPressure, El.status.RestartPressure, El.GetRate(), El.status.DefaultProductionRate,
		El.status.StackVoltage, El.status.Serial)
	return html
}

func getDryerHtmlStatus(El *Electrolyser) (html string) {
	// Check the relay status to ensure power is being provided to the electrolyser
	if El.status.Device == 0 && !SystemStatus.Relays.EL0 {
		html = `<h3 style="text-align:center">Electrolyser/Dryer is switched OFF</h3`
		return
	}

	html = fmt.Sprintf(`<table>
	 <tr><td class="label">Temperature 0</td><td>%0.2f℃</td><td class="label">Temperature 1</td><td>%0.2f℃</td></tr>
	 <tr><td class="label">Temperature 2</td><td>%0.2f℃</td><td class="label">Temperature 3</td><td>%0.2f℃</td></tr>
	 <tr><td class="label">Input Pressure</td><td>%0.2f bar</td><td class="label">Ouput Pressure</td><td>%0.2f bar</td></tr>
	 <tr><td class="label">Dryer Error</td><td>%s</td><td class="label">Dryer Warning</td><td>%s</td></tr>
	</table>`,
		El.status.DryerTemp1, El.status.DryerTemp2, El.status.DryerTemp3, El.status.DryerTemp4,
		El.status.DryerInputPressure, El.status.DryerOutputPressure, El.GetDryerErrorsHTML(), El.GetDryerWarningsHTML())
	return html
}

/**
Get the electrolyser status as a JSON object
*/
func getElectrolyserJsonStatus(w http.ResponseWriter, r *http.Request) {
	// Set the returned type to application/json
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if err == nil {
		err = validateDevice(uint8(device))
	}
	if err != nil {
		log.Print("Invalid electrolyser in status request")
		getStatus(w, r)
		return
	}
	bytesArray, err := json.Marshal(&SystemStatus.Electrolysers[device].status)
	if err != nil {
		log.Println(&SystemStatus.Electrolysers[device].status)
		if _, err := fmt.Fprint(w, errorToJson(err)); err != nil {
			log.Print(err)
		}
	}
	if _, err := fmt.Fprint(w, string(bytesArray)); err != nil {
		log.Print(err)
	}
}

/**
enableElectrolyser sets the electrolyser status to switched on. It can be scheduled using the timeAfterFunc method
*/
func enableElectrolyser(device int) func() {
	return func() {
		SystemStatus.Electrolysers[device].status.SwitchedOn = true
	}
}

/**
Get electrolyser recorded values
*/
func getElectrolyserHistory(w http.ResponseWriter, r *http.Request) {
	type Row struct {
		Logged           string  `json:"logged"`
		El0Rate          float64 `json:"el0Rate"`
		El0Temp          float64 `json:"el0Temp"`
		El0State         int64   `json:"el0State"`
		El0H2Flow        float64 `json:"el0H2Flow"`
		El0InnerPressure float64 `json:"el0InnerPressure"`
		El0OuterPressure float64 `json:"el0OuterPressure"`
		El0StackVoltage  float64 `json:"el0StackVoltage"`
		El0StackCurrent  float64 `json:"El0StackCurrent"`
		El0SystemState   int64   `json:"el0SystemState"`
		El0WaterPressure float64 `json:"el0WaterPressure"`
		DRTemp0          float64 `json:"dryerTemp0"`
		DRTemp1          float64 `json:"dryerTemp1"`
		DRTemp2          float64 `json:"dryerTemp2"`
		DRTemp3          float64 `json:"dryerTemp3"`
		DRInputPressure  float64 `json:"dryerIputPressure"`
		DROutputPressure float64 `json:"dryerOutputPressure"`
		El1Rate          float64 `json:"el1Rate"`
		El1Temp          float64 `json:"el1Temp"`
		El1State         int64   `json:"el1State"`
		El1H2Flow        float64 `json:"el1H2Flow"`
		El1InnerPressure float64 `json:"el1InnerPressure"`
		El1OuterPressure float64 `json:"el1OuterPressure"`
		El1StackVoltage  float64 `json:"el1StackVoltage"`
		El1StackCurrent  float64 `json:"el1StackCurrent"`
		El1SystemState   int64   `json:"el1SystemState"`
		El1WaterPressure float64 `json:"el1WaterPressure"`
		H2Pressure       float64 `json:"h2Pressure"`
	}

	var results []*Row
	// Set the returned type to application/json
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	from := vars["from"]
	to := vars["to"]

	rows, err := pDB.Query(`SELECT (UNIX_TIMESTAMP(logged) DIV 60) * 60, IFNULL(ROUND(AVG(el0Rate)/10,1) ,0), IFNULL(ROUND(AVG(el0ElectrolyteTemp)/10,1) ,0), IFNULL(MAX(el0StateCode) ,0), IFNULL(ROUND(AVG(el0H2Flow)/10,1), 0), IFNULL(ROUND(AVG(el0H2InnerPressure)/10,1), 0),
		IFNULL(ROUND(AVG(el0H2OuterPressure)/10,1), 0), IFNULL(ROUND(AVG(el0StackVoltage)/10,1), 0), IFNULL(ROUND(AVG(el0StackCurrent)/10,1), 0), IFNULL(MAX(el0SystemStateCode), 0), IFNULL(ROUND(AVG(el0WaterPressure)/10,1), 0),
		IFNULL(ROUND(AVG(drTemp0)/10,1), 0), IFNULL(ROUND(AVG(drTemp1)/10,1), 0), IFNULL(ROUND(AVG(drTemp2)/10,1), 0), IFNULL(ROUND(AVG(drTemp3)/10,1), 0), IFNULL(ROUND(AVG(drInputPressure)/10,1), 0), IFNULL(ROUND(AVG(drOutputPressure)/10,1), 0),
		IFNULL(ROUND(AVG(el1Rate)/10,1), 0), IFNULL(ROUND(AVG(el1ElectrolyteTemp)/10,1), 0), IFNULL(MAX(el1StateCode), 0), IFNULL(ROUND(AVG(el1H2Flow)/10,1), 0), IFNULL(ROUND(AVG(el1H2InnerPressure)/10,1), 0),
		IFNULL(ROUND(AVG(el1H2OuterPressure)/10,1), 0), IFNULL(ROUND(AVG(el1StackVoltage)/10,1), 0), IFNULL(ROUND(AVG(el1StackCurrent)/10,1), 0), IFNULL(MAX(el1SystemStateCode), 0), IFNULL(ROUND(AVG(el1WaterPressure)/10,1), 0),
		IFNULL(ROUND(AVG(gasTankPressure)/10,1), 0)
	  FROM firefly.logging
	  WHERE logged BETWEEN ? and ?
	  GROUP BY UNIX_TIMESTAMP(logged) DIV 60`, from, to)
	if err != nil {
		ReturnJSONError(w, "database", err, http.StatusInternalServerError, true)
		return
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err)
		}
	}()
	for rows.Next() {
		row := new(Row)
		if err := rows.Scan(&(row.Logged),
			&(row.El0Rate), &(row.El0Temp), &(row.El0State), &(row.El0H2Flow), &(row.El0InnerPressure), &(row.El0OuterPressure), &(row.El0StackVoltage), &(row.El0StackCurrent), &(row.El0SystemState), &(row.El0WaterPressure),
			&(row.DRTemp0), &(row.DRTemp1), &(row.DRTemp2), &(row.DRTemp3), &(row.DRInputPressure), &(row.DROutputPressure),
			&(row.El1Rate), &(row.El1Temp), &(row.El1State), &(row.El1H2Flow), &(row.El1InnerPressure), &(row.El1OuterPressure), &(row.El1StackVoltage), &(row.El1StackCurrent), &(row.El1SystemState), &(row.El1WaterPressure),
			&(row.H2Pressure)); err != nil {

			log.Print(err)
		} else {
			results = append(results, row)
		}
	}
	if JSON, err := json.Marshal(results); err != nil {
		if _, err := fmt.Fprintf(w, `{"error":"%s"`, err.Error()); err != nil {
			log.Println("Error returning Electrolyser History Error - ", err)
		}
	} else {
		if _, err := fmt.Fprintf(w, string(JSON)); err != nil {
			log.Println("Error returning Electrolyser History - ", err)
		}
	}
}

/**
Turn all electrolysers off
*/
func setAllElOff(w http.ResponseWriter, _ *http.Request) {
	//	log.Println("Setting all electrolysers off")
	err := mbusRTU.EL1OnOff(false)
	if err != nil {
		ReturnJSONError(w, "Electrolyser", err, http.StatusInternalServerError, true)
		return
	}
	err = mbusRTU.EL0OnOff(false)
	if err != nil {
		ReturnJSONError(w, "Electrolyser", err, http.StatusInternalServerError, true)
		return
	}
	returnJSONSuccess(w)
}

/**
Turn the given electrolyser off
*/
func setElOff(w http.ResponseWriter, r *http.Request) {
	var jErr JSONError
	vars := mux.Vars(r)
	device := vars["device"]
	switch device {
	case "0":
		if SystemStatus.Electrolysers[0].status.StackVoltage > 30 {
			log.Println("Electrolyser 1 not turned off because stack voltage is too high.")
			var jErr JSONError
			log.Println(jErr.AddErrorString("Electrolyser", "Electrolyser 1 not turned off because stack voltage is too high."))
			jErr.ReturnError(w, 400)
			return
		}
		if err := mbusRTU.EL0OnOff(false); err != nil {
			ReturnJSONError(w, "Electrolyser-0", err, http.StatusInternalServerError, true)
			return
		}
	case "1":
		if len(SystemStatus.Electrolysers) > 1 {
			if SystemStatus.Electrolysers[1].status.StackVoltage > 30 {
				log.Println(jErr.AddErrorString("Electrolyser", "Electrolyser 2 not turned off because stack voltage is too high."))
				jErr.ReturnError(w, 400)
				return
			}
		}
		if err := mbusRTU.EL1OnOff(false); err != nil {
			ReturnJSONError(w, "Electrolyser-1", err, http.StatusInternalServerError, true)
			return
		}
	default:
		ReturnJSONErrorString(w, "Electrolyser", fmt.Sprintf("Invalid electrolyser specified - %s", device), http.StatusBadRequest, false)
		return
	}
	returnJSONSuccess(w)
}

/**
Turn the given electrolyser on
*/
func setElOn(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device := vars["device"]
	deviceNum, err := strconv.ParseInt(device, 10, 8)
	if err != nil {
		log.Println("Failed to get the device. - ", err)
	}
	if err := validateDevice(uint8(deviceNum)); err != nil {
		log.Println("Invalid device requeted in selElOn - ", err)
	}
	switch deviceNum {
	case 1:
		err = mbusRTU.EL0OnOff(true)
	case 2:
		err = mbusRTU.EL1OnOff(true)
	default:
		ReturnJSONErrorString(w, "Electrolyser", "Invalid electrolyser specified", http.StatusBadRequest, false)
		return
	}
	if err != nil {
		ReturnJSONError(w, "Electrolyser", err, http.StatusInternalServerError, true)
		return
	}
	returnJSONSuccess(w)
}

/**
Turn all electrolysers on
*/
func setAllElOn(w http.ResponseWriter, _ *http.Request) {

	if err := mbusRTU.EL0OnOff(true); err != nil {
		ReturnJSONError(w, "Electrolyser-0", err, http.StatusInternalServerError, true)
		return
	}
	if err := mbusRTU.EL1OnOff(true); err != nil {
		ReturnJSONError(w, "Electrolyser-1", err, http.StatusInternalServerError, true)
		return
	}
	returnJSONSuccess(w)
}

func rebootDryer(w http.ResponseWriter, _ *http.Request) {
	if err := SystemStatus.Electrolysers[0].RebootDryer(); err != nil {
		ReturnJSONError(w, "Dryer", err, http.StatusInternalServerError, true)
		return
	}
	returnJSONSuccess(w)
}
