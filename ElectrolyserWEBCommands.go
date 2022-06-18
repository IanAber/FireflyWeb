package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

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
		log.Println("Failed to get the device. - ", err)
	}
	if err := validateDevice(uint8(deviceNum) - 1); err != nil {
		log.Println("Invalid device requeted in selElOn - ", err)
	}

	SystemStatus.Electrolysers[deviceNum-1].Stop(true)
	if _, err := fmt.Fprintf(w, "Electrolyser stop requested"); err != nil {
		log.Println("Error returning status after electrolyser stop request. - ", err)
	}
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
	if _, err := fmt.Fprintf(w, "Electrolyser reboot requested"); err != nil {
		log.Println("Error returning status after electrolyser reboot request. - ", err)
	}
}

/**
Set a reboot command to all electrolysers
*/
func rebootAllElectrolysers(w http.ResponseWriter, _ *http.Request) {
	for _, el := range SystemStatus.Electrolysers {
		el.Reboot()
	}
	if _, err := fmt.Fprintf(w, "Electrolyser reboot requested"); err != nil {
		log.Println("Error returning status after electrolyser reboot request. - ", err)
	}
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
	var jError JSONError
	if debug {
		log.Println("setElectrolyserRate")
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Print(err)
		var jErr JSONError
		jErr.AddError("electrolyser", err)
		jErr.ReturnError(w, 500)
		return
	}
	var jRate struct {
		Rate int64 `json:"rate"`
	}
	if err = json.Unmarshal(body, &jRate); err != nil {
		log.Println(err)
		return
	}
	if debug {
		log.Printf("Set electrolyser : %d", jRate.Rate)
	}

	_, err = pDB.Exec("INSERT INTO ElectrolyserRequests (RateRequested) VALUES (?)", jRate.Rate)
	if err != nil {
		log.Println("Log Electrolyser Request - ", err)
	}

	// Return bad request if outside acceptable range of 0..100%
	if jRate.Rate > 100 || jRate.Rate < 0 {
		var jErr JSONError
		jErr.AddErrorString("electrolyser", "Rate must be between 0 and 100")
		jErr.ReturnError(w, 400)
		return
	}

	if debug {
		log.Println("Set electrolyser rate to", jRate.Rate, "%")
	}

	var elRates Rate
	if len(SystemStatus.Electrolysers) > 1 {
		elRates = getRates(int8(jRate.Rate))
	} else {
		elRates.el0 = 0
		elRates.el1 = 0
		if jRate.Rate == 0 {
			elRates.el1 = 0
		} else {
			elRates.el1 = uint8((jRate.Rate*4)/10) + 60
		}
	}

	if debug {
		log.Println("set electrolyser 0 to ", elRates.el0)
	}
	err = setElectrolyserRatePercent(elRates.el0, 0)
	if err != nil {
		jError.AddError("el0", err)
	}
	if debug {
		log.Println("set electrolyser 1 to ", elRates.el1)
	}
	if len(SystemStatus.Electrolysers) > 1 {
		err = setElectrolyserRatePercent(elRates.el1, 1)
		if err != nil {
			jError.AddError("el1", err)
		}
	}
	if len(jError.Errors) > 0 {
		if debug {
			log.Println("Errors encountered setting electrolyser rate.")
		}
		jError.ReturnError(w, 500)
	} else {
		if _, err := fmt.Fprintf(w, `{"el0":%d,"el1":%d`, elRates.el0, elRates.el1); err != nil {
			log.Println("Failed to send response -", err)
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
		// If all electrolysers are on get the roduction settings
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
				jReturnData.Rate = int8(((SystemStatus.Electrolysers[0].GetRate() - 60) * 100) / 40)
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
		var jErr JSONError
		jErr.AddError("Electrolyser", err)
		jErr.ReturnError(w, 500)
	} else {
		if _, err := fmt.Fprint(w, string(bytesArray)); err != nil {
			var jErr JSONError
			jErr.AddError("Electrolyser", err)
			jErr.ReturnError(w, 500)
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
		var jErr JSONError
		jErr.AddErrorString("Electrolyser", "Invalid device specified - device was not numeric")
		jErr.ReturnError(w, 400)
		log.Println(err)
		return
	}
	sPressure := vars["bar"]
	pressure, err := strconv.ParseFloat(sPressure, 32)
	if err != nil {
		var jErr JSONError
		jErr.AddErrorString("Electrolyser", "Invalid pressure specified - pressure was not numeric")
		jErr.ReturnError(w, 400)
		log.Print(err)
		return
	}

	if (device < 1) || (device > int64(len(SystemStatus.Electrolysers))) {
		var jErr JSONError
		jErr.AddErrorString("Electrolyser", "Invalid device specified")
		jErr.ReturnError(w, 400)
		return
	}
	if (pressure < 2.0) || (pressure > 35.0) {
		var jErr JSONError
		jErr.AddErrorString("Electrolyser", "Invalid pressure specified (2..35)")
		jErr.ReturnError(w, 400)
		return
	}

	err = SystemStatus.Electrolysers[device-1].SetRestartPressure(float32(pressure))
	if err != nil {
		var jErr JSONError
		jErr.AddError("Electrolyser", err)
		jErr.ReturnError(w, 500)
	}
	if _, err := fmt.Fprintf(w, "OK"); err != nil {
		log.Println("Failed to send response -", err)
	}
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
		State            string  `json:"state"`
		Flow             float32 `json:"flow"`
		H2InnerPressure  float32 `json:"h2InnerPressure"`
		H2OuterPressure  float32 `json:"h2OuterPressure"`
		StackVoltage     float32 `json:"stackVoltage"`
		SystemState      string  `json:"systemState"`
		WaterPressure    float32 `json:"waterPressure"`
		StackCurrent     float32 `json:"stackCurrent"`
	}
	var results []*Row
	var el int32
	var jErr JSONError

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
		log.Println(err)
		jErr.AddError("Electrolyser", err)
		jErr.ReturnError(w, http.StatusBadRequest)
		return
	}
	var sSQL string
	if el == 0 {
		sSQL = `SELECT MIN(UNIX_TIMESTAMP(logged)) AS logged, AVG(el1Rate) AS rate, LAST_VALUE(el1ElectrolyteLevel) AS electrolyteLevel, AVG(el1ElectrolyteTemp) AS electrolyteTemp, LAST_VALUE(el1State) AS state, AVG(el1H2Flow) AS flow, 
AVG(el1H2InnerPressure) AS h2InnerPressure, AVG(el1H2OuterPressure) AS h2OuterPressure, AVG(el1StackVoltage) AS stackVoltage, LAST_VALUE(el1SystemState) AS systemState, AVG(el1WaterPressure) AS waterPressure, AVG(el1StackCurrent) AS stackCurrent
FROM firefly.logging
WHERE el1State IS NOT NULL AND logged BETWEEN ? AND ?
GROUP BY UNIX_TIMESTAMP(logged) DIV ?;`
	} else {
		sSQL = `SELECT MIN(UNIX_TIMESTAMP(logged)) AS logged, AVG(el2Rate) AS rate, LAST_VALUE(el2ElectrolyteLevel) AS electrolyteLevel, AVG(el2ElectrolyteTemp) AS electrolyteTemp, LAST_VALUE(el2State) AS state, AVG(el2H2Flow) AS flow, 
AVG(el2H2InnerPressure) AS h2InnerPressure, AVG(el2H2OuterPressure) AS h2OuterPressure, AVG(el2StackVoltage) AS stackVoltage, LAST_VALUE(el2SystemState) AS systemState, AVG(el2WaterPressure) AS waterPressure, AVG(el2StackCurrent) AS stackCurrent
FROM firefly.logging
WHERE el2State IS NOT NULL AND logged BETWEEN ? AND ?
GROUP BY UNIX_TIMESTAMP(logged) DIV ?;`
	}

	fromTime, err := time.ParseInLocation("2006-1-2T15:4", from, time.Local)
	if err != nil {
		jErr.AddError("Electrolyser", err)
		jErr.ReturnError(w, http.StatusBadRequest)
		return
	}
	toTime, err := time.ParseInLocation("2006-1-2T15:4", to, time.Local)
	if err != nil {
		jErr.AddError("Electrolyser", err)
		jErr.ReturnError(w, http.StatusBadRequest)
		return
	}

	// Get the difference in the two times in nanoseconds
	timeDiff := toTime.Sub(fromTime)
	// We want around 300 points so find out how many seconds per point that would be
	frequency := timeDiff / 300
	rows, err := pDB.Query(sSQL, from, to, frequency.Seconds())
	if err != nil {
		jErr.AddError("FuelCell", err)
		jErr.ReturnError(w, http.StatusInternalServerError)
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
		if err := rows.Scan(&(row.Logged), &(row.Rate), &(row.ElectrolyteLevel), &(row.ElectrolyteTemp), &(row.State), &(row.Flow),
			&(row.H2InnerPressure), &(row.H2OuterPressure), &(row.StackVoltage), &(row.SystemState), &(row.WaterPressure), &(row.StackCurrent)); err != nil {
			jErr.AddError("FuelCell", err)
			jErr.ReturnError(w, http.StatusInternalServerError)
			return
		} else {
			results = append(results, row)
		}
	}
	if len(results) == 0 {
		jErr.AddErrorString("Electrolyser", "No results found - "+from+" | "+to+" | device = "+device+` | `+sSQL)
		jErr.ReturnError(w, http.StatusBadRequest)
		return
	}
	if JSON, err := json.Marshal(results); err != nil {
		jErr.AddError("Electrolyser", err)
		jErr.ReturnError(w, http.StatusInternalServerError)
	} else {
		if _, err := fmt.Fprintf(w, string(JSON)); err != nil {
			log.Println(err)
		}
	}
}
