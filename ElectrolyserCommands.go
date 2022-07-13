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

func elCommand(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Command string `json:"command"`
		device  int
	}

	vars := mux.Vars(r)
	deviceStr := vars["device"]
	var strCommand string

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

	switch body.Command {
	case "on":
		if body.device == 1 {
			strCommand = "el2 on"
		} else {
			strCommand = "el1dr on"
		}
		if _, err := sendCommand(strCommand); err != nil {
			ReturnJSONError(w, "Electrolyser", err, http.StatusInternalServerError, true)
		}
	case "off":
		if body.device == 0 {
			strCommand = "el1dr off"
			if SystemStatus.Electrolysers[0].status.StackVoltage > jsonFloat32(params.ElectrolyserMaxStackVoltsTurnOff) {
				ReturnJSONErrorString(w, "Electrolyser", "Electrolyser 0 not turned off because stack voltage is too high.", http.StatusBadRequest, true)
				return
			}
		} else {
			strCommand = "el2 off"
			if len(SystemStatus.Electrolysers) > 1 {
				if SystemStatus.Electrolysers[1].status.StackVoltage > jsonFloat32(params.ElectrolyserMaxStackVoltsTurnOff) {
					ReturnJSONErrorString(w, "Electrolyser", "Electrolyser 1 not turned off because stack voltage is too high.", http.StatusBadRequest, true)
					return
				}
			}
		}
		if _, err := sendCommand(strCommand); err != nil {
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

/**
Set the given electrolyser to the given rate
*/
func setElectrolyserRatePercent(rate uint8, device uint8) error {
	if SystemStatus.Electrolysers[device].status.SwitchedOn {
		//if rate > 0 {
		//	if SystemStatus.Electrolysers[device].status.ElState == ElIdle && (rate > 0) {
		//		// State is idle so start it first if not in holdoff
		//		if time.Now().After(SystemStatus.Electrolysers[device].OnOffTime.Add(ELECTROLYSERHOLDOFFTIME)) {
		//			log.Println("Start electrolyser ", device)
		//			SystemStatus.Electrolysers[device].Start(false)
		//			SystemStatus.Electrolysers[device].OnOffTime = time.Now()
		//		} else {
		//			log.Println("Electrolyser ", device, " is in hold off so is not starting. Waiting until ", SystemStatus.Electrolysers[device].OnOffTime.Add(ELECTROLYSERHOLDOFFTIME).Format("15:04:05"))
		//		}
		//	}
		//}
		SystemStatus.Electrolysers[device].SetProduction(rate)
	} else {
		// Not switched on so if we are setting to more than 0 fire it up as long as we are below the restart pressure
		if SystemStatus.Gas.TankPressure < ELECTROLYSERRESTARTPRESSURE && rate > 0 {
			strCommand := "el1dr on"
			if device == 1 {
				strCommand = "el2 on"
			}
			if _, err := sendCommand(strCommand); err != nil {
				log.Print(err)
			}
		}
	}
	return nil
}

/**
Tell the given electrolyser to preheat the electrolyte
*/
func preheatElectrolyser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device := vars["device"]
	deviceNum, err := strconv.ParseInt(device, 10, 8)
	if err != nil {
		log.Println("Failed to get the device. - ", err)
	}
	if err := validateDevice(uint8(deviceNum) - 1); err != nil {
		log.Println("Invalid device requeted in preheat - ", err)
	}

	SystemStatus.Electrolysers[deviceNum-1].Preheat()
	if _, err := fmt.Fprintf(w, "Electrolyser %d preheat requested", deviceNum); err != nil {
		log.Println("Error returning status after electrolyser preheat request. - ", err)
	}
}

/**
Tell all electrolysers to preheat the electrolyte
*/
func preheatAllElectrolysers(w http.ResponseWriter, _ *http.Request) {
	for _, el := range SystemStatus.Electrolysers {
		el.Preheat()
	}
	if _, err := fmt.Fprintf(w, "Electrolyser preheat requested"); err != nil {
		log.Println("Error returning status after electrolyser preheat all request. - ", err)
	}
}

/**
Start the given electrolyser
*/
func startElectrolyser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device := vars["device"]
	deviceNum, err := strconv.ParseInt(device, 10, 8)
	if err != nil {
		log.Println("Failed to get the device. - ", err)
	}
	if err := validateDevice(uint8(deviceNum) - 1); err != nil {
		log.Println("Invalid device requeted in selElOn - ", err)
	}

	// Start immediately
	SystemStatus.Electrolysers[deviceNum-1].Start(true)
	if _, err := fmt.Fprintf(w, "Electrolyser start requested"); err != nil {
		log.Println("Error returning status after electrolyser start request. - ", err)
	}
}

/**
Start all electrolysers
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

/**
Stop the given electrolyser immediately
*/
func stopElectrolyser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device := vars["device"]
	deviceNum, err := strconv.ParseInt(device, 10, 8)
	if err != nil {
		ReturnJSONError(w, "Electrolyser", err, http.StatusBadRequest, true)
		return
	}
	if err := validateDevice(uint8(deviceNum) - 1); err != nil {
		ReturnJSONError(w, "Electrolyser", err, http.StatusBadRequest, true)
		return
	}

	SystemStatus.Electrolysers[deviceNum-1].Stop(true)
	returnJSONSuccess(w)
}

/**
Stop all electrolysers
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

/**
Reboot the given electrolyser
*/
func rebootElectrolyser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device := vars["device"]
	deviceNum, err := strconv.ParseInt(device, 10, 8)
	if err != nil {
		log.Println("Failed to get the device. - ", err)
	}
	if err := validateDevice(uint8(deviceNum) - 1); err != nil {
		log.Println("Invalid device requeted in selElOn - ", err)
	}
	SystemStatus.Electrolysers[deviceNum-1].Reboot()
	returnJSONSuccess(w)
}

/**
Set a reboot command to all electrolysers
*/
func rebootAllElectrolysers(w http.ResponseWriter, _ *http.Request) {
	for _, el := range SystemStatus.Electrolysers {
		el.Reboot()
	}
	returnJSONSuccess(w)
}

type Rate struct {
	el0 uint8
	el1 uint8
}

var percentToRate = []Rate{
	{0, 0},   //0
	{60, 0},  //1
	{61, 0},  //2
	{62, 0},  //3
	{63, 0},  //4
	{64, 0},  //5
	{65, 0},  //6
	{66, 0},  //7
	{67, 0},  //8
	{69, 0},  //9
	{70, 0},  //10
	{71, 0},  //11
	{72, 0},  //12
	{74, 0},  //13
	{75, 0},  //14
	{76, 0},  //15
	{77, 0},  //16
	{78, 0},  //17
	{80, 0},  //18
	{81, 0},  //19
	{82, 0},  //20
	{83, 0},  //21
	{84, 0},  //22
	{85, 0},  //23
	{86, 0},  //24
	{87, 0},  //25
	{88, 0},  //26
	{89, 0},  //27
	{90, 0},  //28
	{91, 0},  //29
	{92, 0},  //30
	{93, 0},  //31
	{94, 0},  //32
	{95, 0},  //33
	{96, 0},  //34
	{97, 0},  //35
	{60, 60}, //36
	{61, 60}, //37
	{62, 60}, //38
	{63, 60}, //39
	{64, 60}, //40
	{66, 60}, //41
	{68, 60}, //42
	{69, 60}, //43
	{71, 60}, //44
	{72, 60}, //45
	{73, 60}, //46
	{74, 60}, //47
	{75, 60}, //48
	{76, 60}, //49
	{78, 60}, //50
	{79, 60}, //51
	{80, 60}, //52
	{81, 60}, //53
	{83, 60}, //54
	{84, 60}, //55
	{85, 60}, //56
	{86, 60}, //57
	{87, 60}, //58
	{80, 60}, //59
	{89, 60}, //60
	{90, 60}, //61
	{91, 60}, //62
	{92, 60}, //63
	{93, 60}, //64
	{94, 60}, //65
	{95, 60}, //66
	{96, 60}, //67
	{97, 61}, //68
	{97, 62}, //69
	{97, 63}, //70
	{97, 64}, //71
	{97, 65}, //72
	{97, 66}, //73
	{97, 67}, //74
	{97, 69}, //75
	{97, 70}, //76
	{97, 71}, //77
	{97, 72}, //78
	{97, 73}, //79
	{97, 75}, //80
	{97, 76}, //81
	{97, 77}, //82
	{97, 79}, //83
	{97, 80}, //84
	{97, 81}, //85
	{97, 82}, //86
	{97, 83}, //87
	{97, 85}, //88
	{97, 86}, //89
	{97, 87}, //90
	{97, 88}, //91
	{97, 89}, //92
	{97, 90}, //93
	{97, 91}, //94
	{97, 92}, //95
	{97, 93}, //96
	{97, 94}, //97
	{97, 95}, //98
	{97, 96}, //99
	{97, 97}, //100
}

/**
Translate the given percentage total rates to values suitable for two electrolysers
by looking them up in a MAP
*/
func getRates(rate int8) Rate {

	return percentToRate[rate]
	//for k, v := range RateMap {
	//	if v == rate {
	//		return uint8(k % 1000), uint8(k / 1000)
	//	}
	//}
	//return 0, 0
}

/**
set the electrolyser rate.
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

	if (SystemStatus.Relays.FuelCell1Run || SystemStatus.Relays.FuelCell2Run) && jRate.Rate > 0 {
		// Do not allow the electrolysers to run if one or more fuel cells are also running
		jRate.Rate = 0

		for _, el := range SystemStatus.Electrolysers {
			// Immediate shut down
			el.Stop(true)
		}
		ReturnJSONErrorString(w, "Electrolyser", "One or more Fuel cells are running. All electrolysers are stopped.", http.StatusBadRequest, false)
		return
	}

	debugPrint("Set electrolyser rate to %d%%", jRate.Rate)

	var elRates Rate
	if len(SystemStatus.Electrolysers) > 1 {
		elRates = getRates(int8(jRate.Rate))
	} else {
		elRates.el0 = 0
		elRates.el1 = 0
		if jRate.Rate == 0 {
			elRates.el0 = 0
		} else {
			elRates.el0 = uint8((jRate.Rate*4)/10) + 60
		}
	}

	debugPrint("set electrolyser 0 to %d%%", elRates.el0)

	err = setElectrolyserRatePercent(elRates.el0, 0)
	if err != nil {
		ReturnJSONError(w, "Electrolyser", err, http.StatusInternalServerError, true)
		return
	}
	if len(SystemStatus.Electrolysers) > 1 {
		debugPrint("set electrolyser 1 to ", elRates.el1)
		err = setElectrolyserRatePercent(elRates.el1, 1)
		if err != nil {
			ReturnJSONError(w, "Electrolyser", err, http.StatusInternalServerError, true)
			return
		}
	}
	if responseBytes, err := json.Marshal(responseJson); err != nil {
		ReturnJSONError(w, "Electrolyser", err, http.StatusInternalServerError, true)
	} else {
		if _, err = fmt.Fprintf(w, string(responseBytes)); err != nil {
			log.Println(err)
		}
	}
}

/**
Return the total electrolyser rate as a percentage, 0-100%
*/
func getElectrolyserRate(w http.ResponseWriter, _ *http.Request) {

	var el1, el2 int
	var jReturnData struct {
		Rate   int8    `json:"rate"`
		Gas    float64 `json:"gas"`
		Status string  `json:"status"`
	}

	// Set the gas pressure
	jReturnData.Gas = SystemStatus.Gas.TankPressure

	// Loop through and find if the electrolysers are on
	ElectrolysersSwitchedOn := len(SystemStatus.Electrolysers) > 0
	for _, e := range SystemStatus.Electrolysers {
		if !e.IsSwitchedOn() {
			// If anyone is switche off then assume all are off
			ElectrolysersSwitchedOn = false
		}
	}
	if ElectrolysersSwitchedOn {
		// If all electrolysers are on get the production settings
		if len(SystemStatus.Electrolysers) == 1 {
			// We only have one electrolyser so this is easy
			switch SystemStatus.Electrolysers[0].status.ElState {
			case ElIdle:
				jReturnData.Status = "Idle"
				jReturnData.Rate = 0
			case ElStandby:
				jReturnData.Status = "Standby"
				jReturnData.Rate = 0
			default:
				jReturnData.Status = "Active"
				r := SystemStatus.Electrolysers[0].GetRate()
				if r > 0 {
					jReturnData.Rate = int8(((r - 60) * 100) / 40)
					if jReturnData.Rate < 1 {
						jReturnData.Rate = 1
					}
					if jReturnData.Rate > 100 {
						jReturnData.Rate = 100
					}
				} else {
					jReturnData.Rate = 0
				}
			}
		} else {
			// We have two electrolysers, so we need to work out the rate using the mapping table
			switch SystemStatus.Electrolysers[0].status.ElState {
			case ElIdle:
				jReturnData.Status = "Idle"
				el1 = 0
			case ElStandby:
				jReturnData.Status = "Standby"
				el1 = 0
			default:
				jReturnData.Status = "Active"
				el1 = int(SystemStatus.Electrolysers[0].GetRate())
			}

			switch SystemStatus.Electrolysers[1].status.ElState {
			case ElIdle:
				if jReturnData.Status != "Standby" {
					jReturnData.Status = "Idle"
				}
				el2 = 0
			case ElStandby:
				jReturnData.Status = "Standby"
				el2 = 0
			default:
				if jReturnData.Status != "Standby" {
					jReturnData.Status = "Idle"
				}
				jReturnData.Status = "Active"
				el2 = int(SystemStatus.Electrolysers[1].GetRate())
			}
			if el1 > 96 {
				el1 = 100
			}
			if el2 > 96 {
				el2 = 100
			}
			jReturnData.Rate = RateMap[(el2*1000)+el1]
		}
	} else {
		// One or more electrolysers are off, so we report as OFF.
		jReturnData.Rate = -1
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

	if (device < 1) || (device > int64(len(SystemStatus.Electrolysers))) {
		ReturnJSONErrorString(w, "Electrolyser", "Invalid device specified", http.StatusBadRequest, true)
		return
	}
	if (pressure < 2.0) || (pressure > 35.0) {
		ReturnJSONErrorString(w, "Electrolyser", "Invalid pressure specified (2..35)", http.StatusBadRequest, true)
		return
	}

	err = SystemStatus.Electrolysers[device-1].SetRestartPressure(float32(pressure))
	if err != nil {
		ReturnJSONError(w, "Electrolyser", err, http.StatusInternalServerError, true)
		return
	}
	returnJSONSuccess(w)
}

/**
Returns a form allowing manual setting of the electrolyser rate
*/
func showRateSetter(w http.ResponseWriter, _ *http.Request) {
	_, err := fmt.Fprint(w, `<html>
	<head>
		<title>Set Electrolyser Rate</title>
		<script type="text/javascript">
function postVal(){ 
            // Creating a XHR object
            let xhr = new XMLHttpRequest();
            let url = "/el/setrate";
       
            // open a connection
            xhr.open("POST", url, true);
 
            // Set the request header i.e. which type of content we are sending
            xhr.setRequestHeader("Content-Type", "application/json");
 
            // Create a state change callback
            xhr.onreadystatechange = function () {
                if (xhr.readyState === 4 && xhr.status === 200) {
 
                    // Print received data from server
                    result.innerHTML = this.responseText;
 
                }
            };
 
            // Converting JSON data to string
            var data = JSON.stringify({ "rate": parseInt(document.getElementById("rate").value) });
 
            // Sending data with the request
            xhr.send(data);
        }
		</script>
	</head>
	<body>
		<div>
			<label for="rate">Rate</label><input id="rate" name="rate" type="number" min="0" max="100" step="1" value="0" /><br />
			<input type="button" onclick="postVal()" value="Submit" />
		</div>
	</body>
</html>`)
	if err != nil {
		log.Println("Failed to send the reate setter form -", err)
	}
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
		sSQL = `SELECT MIN(UNIX_TIMESTAMP(logged)) AS logged, AVG(el1Rate) AS rate, LAST_VALUE(el1ElectrolyteLevel) AS electrolyteLevel, AVG(el1ElectrolyteTemp) AS electrolyteTemp, AVG(el1H2Flow) AS flow, 
AVG(el1H2InnerPressure) AS h2InnerPressure, AVG(el1H2OuterPressure) AS h2OuterPressure, AVG(el1StackVoltage) AS stackVoltage, AVG(el1WaterPressure) AS waterPressure, AVG(el1StackCurrent) AS stackCurrent
FROM firefly.logging
WHERE el1Rate is not null AND logged BETWEEN ? AND ?
GROUP BY UNIX_TIMESTAMP(logged) DIV ?;`
	} else {
		sSQL = `SELECT MIN(UNIX_TIMESTAMP(logged)) AS logged, AVG(el2Rate) AS rate, LAST_VALUE(el2ElectrolyteLevel) AS electrolyteLevel, AVG(el2ElectrolyteTemp) AS electrolyteTemp, AVG(el2H2Flow) AS flow, 
AVG(el2H2InnerPressure) AS h2InnerPressure, AVG(el2H2OuterPressure) AS h2OuterPressure, AVG(el2StackVoltage) AS stackVoltage, AVG(el2WaterPressure) AS waterPressure, AVG(el2StackCurrent) AS stackCurrent
FROM firefly.logging
WHERE el2Rate is not null AND logged BETWEEN ? AND ?
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

func validateDevice(device uint8) error {
	if device >= uint8(systemConfig.NumEl) {
		return fmt.Errorf("invalid Electrolyser device - %d", device)
	}
	return nil
}

func getElectrolyserHtmlStatus(El *Electrolyser) (html string) {
	// Check the relay status to ensure power is being provided to the electrolyser
	if (El.status.Device == 0 && !SystemStatus.Relays.Electrolyser1) || (El.status.Device == 1 && !SystemStatus.Relays.Electrolyser2) {
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
	if (El.status.Device == 0 && !SystemStatus.Relays.Electrolyser1) || (El.status.Device == 1 && !SystemStatus.Relays.Electrolyser2) {
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
	if (err != nil) || (device > 1) || (device < 0) {
		log.Print("Invalid electrolyser in status request")
		getStatus(w, r)
		return
	}
	bytesArray, err := json.Marshal(SystemStatus.Electrolysers[device].status)
	if err != nil {
		log.Println(SystemStatus.Electrolysers[device].status)
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
		Logged            string
		EL1Rate           float64
		EL1Temp           float64
		EL1State          int64
		EL1H2Flow         float64
		EL1InnerPressure  float64
		EL1OuterPressure  float64
		EL1StackVoltage   float64
		EL1StackCurrent   float64
		EL1SystemState    int64
		EL1WaterPressure  float64
		DR1Temp0          float64
		DR1Temp1          float64
		DR1Temp2          float64
		DR1Temp3          float64
		DR1InputPressure  float64
		DR1OutputPressure float64
		EL2Rate           float64
		EL2Temp           float64
		EL2State          int64
		EL2H2Flow         float64
		EL2InnerPressure  float64
		EL2OuterPressure  float64
		EL2StackVoltage   float64
		EL2StackCurrent   float64
		EL2SystemState    int64
		EL2WaterPressure  float64
		DR2Temp0          float64
		DR2Temp1          float64
		DR2Temp2          float64
		DR2Temp3          float64
		DR2InputPressure  float64
		DR2OutputPressure float64
		H2Pressure        float64
	}

	var results []*Row
	// Set the returned type to application/json
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	from := vars["from"]
	to := vars["to"]
	rows, err := pDB.Query(`SELECT (UNIX_TIMESTAMP(logged) DIV 60) * 60, IFNULL(AVG(el1Rate) ,0), IFNULL(AVG(el1ElectrolyteTemp) ,0), IFNULL(MAX(el1StateCode) ,0), IFNULL(AVG(el1H2Flow), 0), IFNULL(AVG(el1H2InnerPressure), 0),
		IFNULL(AVG(el1H2OuterPressure), 0), IFNULL(AVG(el1StackVoltage), 0), IFNULL(AVG(el1StackCurrent), 0), IFNULL(MAX(el1SystemStateCode), 0), IFNULL(AVG(el1WaterPressure), 0),
		IFNULL(AVG(dr1Temp0), 0), IFNULL(AVG(dr1Temp1), 0), IFNULL(AVG(dr1Temp2), 0), IFNULL(AVG(dr1Temp3), 0), IFNULL(AVG(dr1InputPressure), 0), IFNULL(AVG(dr1OutputPressure), 0),
		IFNULL(AVG(el2Rate), 0), IFNULL(AVG(el2ElectrolyteTemp), 0), IFNULL(MAX(el2StateCode), 0), IFNULL(AVG(el2H2Flow), 0), IFNULL(AVG(el2H2InnerPressure), 0),
		IFNULL(AVG(el2H2OuterPressure), 0), IFNULL(AVG(el2StackVoltage), 0), IFNULL(AVG(el2StackCurrent), 0), IFNULL(MAX(el2SystemStateCode), 0), IFNULL(AVG(el2WaterPressure), 0),
		IFNULL(AVG(dr2Temp0), 0), IFNULL(AVG(dr2Temp1), 0), IFNULL(AVG(dr2Temp2), 0), IFNULL(AVG(dr2Temp3), 0), IFNULL(AVG(dr2InputPressure), 0), IFNULL(AVG(dr2OutputPressure), 0),
		IFNULL(AVG(gasTankPressure), 0)
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
			&(row.EL1Rate), &(row.EL1Temp), &(row.EL1State), &(row.EL1H2Flow), &(row.EL1InnerPressure), &(row.EL1OuterPressure), &(row.EL1StackVoltage), &(row.EL1StackCurrent), &(row.EL1SystemState), &(row.EL1WaterPressure),
			&(row.DR1Temp0), &(row.DR1Temp1), &(row.DR1Temp2), &(row.DR1Temp3), &(row.DR1InputPressure), &(row.DR1OutputPressure),
			&(row.EL2Rate), &(row.EL2Temp), &(row.EL2State), &(row.EL2H2Flow), &(row.EL2InnerPressure), &(row.EL2OuterPressure), &(row.EL2StackVoltage), &(row.EL2StackCurrent), &(row.EL2SystemState), &(row.EL2WaterPressure),
			&(row.DR2Temp0), &(row.DR2Temp1), &(row.DR2Temp2), &(row.DR2Temp3), &(row.DR2InputPressure), &(row.DR2OutputPressure),
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
	_, err := sendCommand("el2 off")
	if err != nil {
		ReturnJSONError(w, "Electrolyser", err, http.StatusInternalServerError, true)
		return
	}
	_, err = sendCommand("el1dr off")
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
	var strCommand string
	switch device {
	case "0":
		strCommand = "el1dr off"
		if SystemStatus.Electrolysers[0].status.StackVoltage > 30 {
			log.Println("Electrolyser 1 not turned off because stack voltage is too high.")
			var jErr JSONError
			log.Println(jErr.AddErrorString("Electrolyser", "Electrolyser 1 not turned off because stack voltage is too high."))
			jErr.ReturnError(w, 400)
			return
		}
	case "1":
		strCommand = "el2 off"
		if len(SystemStatus.Electrolysers) > 1 {
			if SystemStatus.Electrolysers[1].status.StackVoltage > 30 {
				log.Println(jErr.AddErrorString("Electrolyser", "Electrolyser 2 not turned off because stack voltage is too high."))
				jErr.ReturnError(w, 400)
				return
			}
		}
	default:
		log.Println(jErr.AddErrorString("Electrolyser", fmt.Sprintf("Invalid electrolyser specified - ", device)))
		jErr.ReturnError(w, 400)
		return
	}
	_, err := sendCommand(strCommand)
	if err != nil {
		log.Println(jErr.AddError("Electrolyser", err))
		jErr.ReturnError(w, 500)
		return
	}
	returnJSONSuccess(w)
}

/**
Turn the given electrolyser on
*/
func setElOn(w http.ResponseWriter, r *http.Request) {
	var jErr JSONError
	vars := mux.Vars(r)
	device := vars["device"]
	var strCommand string
	deviceNum, err := strconv.ParseInt(device, 10, 8)
	if err != nil {
		log.Println("Failed to get the device. - ", err)
	}
	if err := validateDevice(uint8(deviceNum)); err != nil {
		log.Println("Invalid device requeted in selElOn - ", err)
	}
	switch deviceNum {
	case 1:
		strCommand = "el1dr on"
	case 2:
		strCommand = "el2 on"
	default:
		log.Print("Invalid electrolyser specified - ", device)
		getStatus(w, r)
		return
	}
	_, err = sendCommand(strCommand)
	if err != nil {
		log.Println(jErr.AddError("Electrolyser", err))
		jErr.ReturnError(w, 500)
		return
	}
	returnJSONSuccess(w)
}

/**
Turn all electrolysers on
*/
func setAllElOn(w http.ResponseWriter, _ *http.Request) {
	var jErr JSONError
	_, err := sendCommand("el1dr on")
	if err != nil {
		log.Print(jErr.AddError("Electrolyser", err))
		jErr.ReturnError(w, 500)
		return
	}
	_, err = sendCommand("el2 on")
	if err != nil {
		log.Print(jErr.AddError("Electrolyser", err))
		jErr.ReturnError(w, 500)
		return
	}
	returnJSONSuccess(w)
}
