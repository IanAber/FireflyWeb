package main

/*****************************************
This project uses the firefly esm command line interface to control the system components.

*/

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"io"
	"log"
	"math"
	//	"log/syslog"
	syslog "github.com/RackSec/srslog"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const ELECTROLYSERHOLDOFFTIME = time.Minute * 10 // Minimum time after turning off before it is turned on again
const ELECTROLYSERHOLDONTIME = time.Minute * 10  // Minimum time after turning on before it is turned off again
const ELECTROLYSERRESTARTPRESSURE = 33
const ELECTROLYSEROFFDELAYTIME = time.Minute * 3
const ELECTROLYSERSHUTDOWNDELAY = time.Minute * 15
const MAXFUELCELLRESTARTS = 5
const OFFTIMEFORFUELCELLRESTART = time.Second * 90
const convertPSIToBar = 14.503773773
const CANDUMPSQL = `SELECT UNIX_TIMESTAMP(logged) AS logged, ID, canData FROM CAN_Data WHERE logged BETWEEN ? AND ?`
const CANDUMPEVENTSQL = `SELECT UNIX_TIMESTAMP(logged) AS logged, ID, canData FROM CAN_Data WHERE Event = ?`
const LISTCANEVENTSSQL = `SELECT DISTINCT event FROM CAN_Data WHERE event IS NOT NULL ORDER BY event DESC LIMIT 50`
const redirectToMainMenuScript = `
<script>
	var tID = setTimeout(function () {
		window.location.href = "/status";
		window.clearTimeout(tID);		// clear time out.
	}, 2000);
</script>
`

var params *JsonSettings

var RateMap = map[int]int8{
	0:      0,
	60:     1,
	61:     2,
	62:     2,
	63:     3,
	64:     4,
	65:     5,
	66:     6,
	67:     7,
	68:     7,
	69:     8,
	70:     9,
	71:     10,
	72:     11,
	73:     11,
	74:     12,
	75:     13,
	76:     14,
	77:     15,
	78:     16,
	79:     16,
	80:     17,
	81:     18,
	82:     19,
	83:     20,
	84:     21,
	85:     22,
	86:     23,
	87:     24,
	88:     24,
	89:     26,
	90:     27,
	91:     28,
	92:     29,
	93:     30,
	94:     31,
	95:     32,
	96:     33,
	100:    34,
	60060:  34,
	60061:  35,
	60062:  36,
	60063:  37,
	60064:  38,
	60065:  39,
	60066:  39,
	60067:  40,
	60068:  41,
	60069:  42,
	60070:  43,
	60071:  43,
	60072:  44,
	60073:  45,
	60074:  46,
	60075:  47,
	60076:  48,
	60077:  48,
	60078:  49,
	60079:  50,
	60080:  51,
	60081:  52,
	60082:  52,
	60083:  53,
	60084:  54,
	60085:  55,
	60086:  56,
	60087:  56,
	60088:  57,
	60089:  58,
	60090:  59,
	60091:  60,
	60092:  61,
	60093:  62,
	60094:  63,
	60095:  64,
	60096:  65,
	60100:  66,
	61100:  67,
	62100:  68,
	63100:  69,
	64100:  70,
	65100:  71,
	66100:  72,
	67100:  73,
	68100:  74,
	69100:  75,
	70100:  75,
	71100:  76,
	72100:  77,
	73100:  78,
	74100:  79,
	75100:  80,
	76100:  80,
	77100:  81,
	78100:  82,
	79100:  83,
	80100:  84,
	81100:  84,
	82100:  85,
	83100:  86,
	84100:  87,
	85100:  88,
	86100:  89,
	87100:  90,
	88100:  91,
	89100:  92,
	90100:  93,
	91100:  94,
	92100:  95,
	93100:  96,
	94100:  97,
	95100:  98,
	96100:  99,
	100100: 100,
	100060: 1,
	100061: 2,
	100062: 3,
	100063: 3,
	100064: 4,
	100065: 5,
	100066: 6,
	100067: 6,
	100068: 7,
	100069: 8,
	100070: 9,
	100071: 10,
	100072: 11,
	100073: 11,
	100074: 13,
	100075: 14,
	100076: 15,
	100077: 15,
	100078: 16,
	100079: 17,
	100080: 18,
	100081: 18,
	100082: 19,
	100083: 20,
	100084: 21,
	100085: 22,
	100086: 22,
	100087: 23,
	100088: 24,
	100089: 25,
	100090: 25,
	100091: 26,
	100092: 27,
	100093: 28,
	100094: 29,
	100095: 29,
	100096: 30,
}

type gasStatus struct {
	FuelCellPressure float64
	TankPressure     float64
}

//type fuelCellStatus struct {
//	SerialNumber  string
//	Version       string
//	OutputPower   float64
//	OutputVolt    float64
//	OutputCurrent float64
//	AnodePress    float64
//	InletTemp     float64
//	OutletTemp    float64
//	State         string
//	FaultFlagA    string
//	FaultFlagB    string
//	FaultFlagC    string
//	FaultFlagD    string
//	faultTime     time.Time
//	clearTime     time.Time
//	inRestart     bool
//	numRestarts   int
//}

type relayStatus struct {
	FuelCell1Enable bool
	FuelCell1Run    bool
	FuelCell2Enable bool
	FuelCell2Run    bool
	Drain           bool
	Electrolyser1   bool
	Electrolyser2   bool
	GasToFuelCell   bool
}

type tdsStatus struct {
	TdsReading int64
}

var (
	executable       string //This is the esm executable to control the Firefly.
	databaseServer   string
	databasePort     string
	databaseName     string
	databaseLogin    string
	databasePassword string
	CANInterface     string
	pDB              *sql.DB
	esmPrompt        = []byte{27, 91, 51, 50, 109, 13, 70, 73, 82, 69, 70, 76, 89, 27, 91, 51, 57, 109, 32, 62}
	signal           *sync.Cond
	commandResponse  chan string
	esmCommand       struct {
		command *exec.Cmd
		stdin   io.WriteCloser
		stdout  io.ReadCloser
		valid   bool
		mux     sync.Mutex
	}

	SystemStatus struct {
		m      sync.Mutex
		valid  bool
		Relays relayStatus
		//		FuelCells     []*fuelCellStatus
		Electrolysers []*Electrolyser
		Gas           gasStatus
		TDS           tdsStatus
	}

	canBus                   *CANBus
	debug                    bool
	jsonSettings             string
	electrolyserShutDownTime time.Time
)

var systemConfig struct {
	consoleHistory             int64
	NumDryer                   uint16
	NumEl                      uint16
	ElAddresses                string
	NumFc                      uint16
	FcAndElOk                  bool
	IgnoreElState              bool
	SolarArrayInstalled        bool
	SolarMeterMax              int16
	SolarMeterMin              int16
	SolarAveragingTime         int16
	SolarMonitorInterval       int16
	GenH2StatusCheckDelay      int16
	H2FcPressureMin            int16
	FcMonitorInterval          int16
	ElStateQueryWait           int16
	FcInfoWait                 int16
	GenH2ElDetectTimeout       int16
	GenH2ElId                  int16
	LabjackPrecision           int16
	MaxDryerTemp               int16
	MaxElTemp                  int16
	ProductionRateMinThreshold float32
	RunFCElId                  int16
	LogOverwrite               int16
	SimMode                    int16
}

var settings Settings

func connectToDatabase() (*sql.DB, error) {
	if pDB != nil {
		_ = pDB.Close()
		pDB = nil
	}
	// Set the time zone to Local to correctly record times
	var sConnectionString = databaseLogin + ":" + databasePassword + "@tcp(" + databaseServer + ":" + databasePort + ")/" + databaseName + "?loc=Local"

	db, err := sql.Open("mysql", sConnectionString)
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		_ = db.Close()
		pDB = nil
		return nil, err
	}
	return db, err
}

func showElectrolyserProductionRatePage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 2) || (device < 1) {
		log.Print("Invalid electrolyser in set production rate request")
		var jErr JSONError
		jErr.AddErrorString("electrolyser", "Invalid electrolyser in set production rate request")
		jErr.ReturnError(w, 400)
		return
	}
	currentRate := int8(SystemStatus.Electrolysers[device-1].GetRate())
	if _, err := fmt.Fprintf(w, `<html>
  <head>
    <title>Select the required production rate</title>
  </head>
  <body>
    <h1>Select Desired Production Rate For Electrolyser %d</h1>
	<form action="/el/%d/rate" method="post">
      <label for="rate">Production Rate</label>
      <select id="rate" name="rate"><option value=0>Off</option>`, device, device); err != nil {
		log.Println(err)
		return
	}
	for v := int8(100); v > 59; v-- {
		enabled := ""
		if v == currentRate {
			enabled = "selected"
		}
		if _, err := fmt.Fprintf(w, `<option value=%d %s>%d</option>`, v, enabled, v); err != nil {
			log.Println(err)
			return
		}
	}
	if _, err := fmt.Fprintf(w, `</select><br /><input type="submit" value="Set" />
    </form>
  </body>
</html>`); err != nil {
		log.Println(err)
	}
}

func notNumeric(c rune) bool {
	return !unicode.IsNumber(c) && (c != '-') && (c != '.')
}

func getKeyValueLines(text string, valueDelimiter string) []string {
	lines := strings.Split(text, "\n")
	var line string
	var valueLines []string
	for _, line = range lines {
		if strings.Contains(line, valueDelimiter) {
			valueLines = append(valueLines, line)
		}
	}
	return valueLines
}

//func populateFuelCellData(text string, fcStatus *fuelCellStatus) *fuelCellStatus {
//	valueLines := getKeyValueLines(text, ": ")
//	// Clear all existing values becuase we don't necessarily get everything from the fuel cell
//	fcStatus.FaultFlagA = ""
//	fcStatus.FaultFlagB = ""
//	fcStatus.FaultFlagC = ""
//	fcStatus.FaultFlagD = ""
//	fcStatus.State = ""
//	fcStatus.SerialNumber = ""
//	fcStatus.OutputPower = 0.0
//	fcStatus.OutputVolt = 0.0
//	fcStatus.OutputCurrent = 0.0
//	fcStatus.OutletTemp = 0.0
//	fcStatus.InletTemp = 0.0
//	fcStatus.AnodePress = 0.0
//	fcStatus.Version = ""
//
//	// Check that we got something
//	if len(valueLines) > 0 {
//		// For each line, parse the value into the status struct
//		for _, valueLine := range valueLines {
//			keyValue := strings.Split(valueLine, ": ")
//			key := strings.Trim(keyValue[0], " ")
//			value := strings.Trim(keyValue[1], " ")
//			switch key {
//			case "Serial Number":
//				fcStatus.SerialNumber = strings.Trim(value, "\u0000")
//			case "Version":
//				fcStatus.Version = value
//			case "Output Power":
//				fcStatus.OutputPower, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 32)
//			case "Output Volt":
//				fcStatus.OutputVolt, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 32)
//			case "Output Current":
//				fcStatus.OutputCurrent, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 32)
//			case "Anode Press":
//				fcStatus.AnodePress, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 32)
//			case "Inlet Temp":
//				fcStatus.InletTemp, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 32)
//			case "Outlet Temp":
//				fcStatus.OutletTemp, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 32)
//			case "State":
//				fcStatus.State = value
//			case "Fault Flag_A":
//				fcStatus.FaultFlagA = value
//			case "Fault Flag_B":
//				fcStatus.FaultFlagB = value
//			case "Fault Flag_C":
//				fcStatus.FaultFlagC = value
//			case "Fault Flag_D":
//				fcStatus.FaultFlagD = value
//			default:
//				// Don't know how to handle this
//				log.Printf("Fuelcell info returned >>>>> [%s]\n", valueLine)
//			}
//		}
//	}
//	// Return the completed status struct
//	return fcStatus
//}

func populateGasData(text string) {
	valueLines := getKeyValueLines(text, ": ")
	if len(valueLines) > 0 {
		for _, valueLine := range valueLines {
			keyValue := strings.Split(valueLine, ":")
			if len(keyValue) < 2 {
				return
			}
			key := strings.Trim(keyValue[0], " ")
			value := strings.Trim(keyValue[1], " ")
			switch key {
			case "Fuel Cell pressure":
				SystemStatus.Gas.FuelCellPressure, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 64)
				SystemStatus.Gas.FuelCellPressure /= convertPSIToBar
			case "Tank pressure":
				SystemStatus.Gas.TankPressure, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 64)
				SystemStatus.Gas.TankPressure /= convertPSIToBar
			}
		}
	}
	return
}

func isOn(val string) bool {
	if strings.Contains(val, "ON") {
		return true
	} else if !strings.Contains(val, "OFF") {
		log.Println("Didn't understand the boolean value", val)
	}
	return false
}

func populateSystemInfo(text string) {
	valueLines := getKeyValueLines(text, " = ")
	systemConfig.NumDryer = 0
	systemConfig.NumFc = 0
	systemConfig.NumEl = 0
	if len(valueLines) > 0 {
		for _, valueLine := range valueLines {
			keyValue := strings.Split(valueLine, " = ")
			if len(keyValue) < 2 {
				return
			}
			key := strings.Trim(keyValue[0], " ")
			value := strings.Trim(keyValue[1], " ")
			switch key {
			case "Num_Dryer":
				n, _ := strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 16)
				systemConfig.NumDryer = uint16(n)
			case "Num_EL":
				n, _ := strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 16)
				systemConfig.NumEl = uint16(n)

			case "Num_FC":
				n, _ := strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 16)
				systemConfig.NumFc = uint16(n)
				//for device := int64(0); device < n; device++ {
				//	SystemStatus.FuelCells = append(SystemStatus.FuelCells, new(fuelCellStatus))
				//}
			case "EL_Addresses":
				systemConfig.ElAddresses = value
			case "FC_and_EL_OK":
				systemConfig.FcAndElOk = value == "True"
			case "Ignore_EL_State":
				systemConfig.IgnoreElState = value == "True"
			case "Solar_Array_Installed":
				systemConfig.SolarArrayInstalled = value == "True"
			case "Solar_Meter_Max":
				n, _ := strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 16)
				systemConfig.SolarMeterMax = int16(n)
			case "Solar_Meter_Min":
				n, _ := strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 16)
				systemConfig.SolarMeterMin = int16(n)
			case "Solar_Averaging_Time":
				n, _ := strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 16)
				systemConfig.SolarAveragingTime = int16(n)
			case "Solar_Monitor_Interval":
				n, _ := strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 16)
				systemConfig.SolarMonitorInterval = int16(n)
			case "Gen_H2_status_check_delay":
				n, _ := strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 16)
				systemConfig.GenH2StatusCheckDelay = int16(n)
			case "H2_FC_Pressure_Min":
				n, _ := strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 16)
				systemConfig.H2FcPressureMin = int16(n)
			case "FC_Monitor_Interval":
				n, _ := strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 16)
				systemConfig.FcMonitorInterval = int16(n)
			case "EL_State_Query_Wait":
				n, _ := strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 16)
				systemConfig.ElStateQueryWait = int16(n)
			case "FC_Info_Wait":
				n, _ := strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 16)
				systemConfig.FcInfoWait = int16(n)
			case "GenH2_EL_Detect_timeout":
				n, _ := strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 16)
				systemConfig.GenH2ElDetectTimeout = int16(n)
			case "GenH2_El_id":
				n, _ := strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 16)
				systemConfig.GenH2ElId = int16(n)
			case "Labjack_precision":
				n, _ := strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 16)
				systemConfig.LabjackPrecision = int16(n)
			case "Max_Dryer_Temp":
				n, _ := strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 16)
				systemConfig.MaxDryerTemp = int16(n)
			case "Max_EL_Temp":
				n, _ := strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 16)
				systemConfig.MaxElTemp = int16(n)
			case "Production_rate_min_threshold":
				f, _ := strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 32)
				systemConfig.ProductionRateMinThreshold = float32(f)
			case "RunFC_El_id":
				n, _ := strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 16)
				systemConfig.RunFCElId = int16(n)
			case "console_history":
				n, _ := strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 16)
				systemConfig.consoleHistory = n
			}
		}
	}
	if systemConfig.NumEl > 0 {
		addresses := strings.Split(systemConfig.ElAddresses, ",")
		SystemStatus.Electrolysers = append(SystemStatus.Electrolysers, NewElectrolyser(strings.Trim(addresses[0], " ")))
		if systemConfig.NumEl > 1 {
			SystemStatus.Electrolysers = append(SystemStatus.Electrolysers, NewElectrolyser(strings.Trim(addresses[1], " ")))
		}
	}

	return
}

func getSystemInfo() {
	text, err := sendCommand("conf show")
	if err != nil {
		log.Fatal(err)
	}
	populateSystemInfo(text)
}

func populateRelayData(text string) bool {
	valueLines := getKeyValueLines(text, ": ")
	type relaysFound struct {
		enab1 bool
		run1  bool
		enab2 bool
		run2  bool
		drain bool
		el1dr bool
		el2   bool
		gas   bool
	}

	relays := relaysFound{false, false, false, false, false, false, false, false}
	if len(valueLines) > 0 {
		for _, valueLine := range valueLines {
			keyValue := strings.Split(valueLine, ":")
			if len(keyValue) < 2 {
				return false
			}
			key := strings.Trim(keyValue[0], " ")
			value := strings.Trim(keyValue[1], " ")
			switch key {
			//goland:noinspection ALL
			case "Enab1":
				relays.enab1 = true
				SystemStatus.Relays.FuelCell1Enable = isOn(value)
			case "Run1":
				relays.run1 = true
				SystemStatus.Relays.FuelCell1Run = isOn(value)
			case "Enab2":
				relays.enab2 = true
				SystemStatus.Relays.FuelCell2Enable = isOn(value)
			case "Run2":
				relays.run2 = true
				SystemStatus.Relays.FuelCell2Run = isOn(value)
			case "Drain":
				relays.drain = true
				SystemStatus.Relays.Drain = isOn(value)
			case "El1Dr":
				relays.el1dr = true
				SystemStatus.Relays.Electrolyser1 = isOn(value)
			case "El2":
				relays.el2 = true
				SystemStatus.Relays.Electrolyser2 = isOn(value)
			case "Gas":
				relays.gas = true
				SystemStatus.Relays.GasToFuelCell = isOn(value)
			}
		}
	}
	return relays.gas && relays.el1dr && relays.el2 && relays.drain && relays.enab1 && relays.enab2 && relays.run1 && relays.run2
}

func populateTdsData(text string) {
	valueLines := getKeyValueLines(text, ": ")
	if len(valueLines) > 0 {
		for _, valueLine := range valueLines {
			keyValue := strings.Split(valueLine, ":")
			if len(keyValue) < 2 {
				return
			}
			key := strings.Trim(keyValue[0], " ")
			value := strings.Trim(keyValue[1], " ")
			switch key {
			case "TDS reading":
				SystemStatus.TDS.TdsReading, _ = strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 64)
			}
		}
	}
	return
}

func validateDevice(device uint8) error {
	if device >= uint8(systemConfig.NumEl) {
		return fmt.Errorf("invalid Electrolyser device - %d", device)
	}
	return nil
}

// Send the command via the esm command line application
func sendCommand(commandString string) (string, error) {
	var responseText string

	// Ensure that the command is allowed to complete before any other command is started.
	esmCommand.mux.Lock()
	defer esmCommand.mux.Unlock()

	// Send the command to the esm application
	_, err := fmt.Fprintln(esmCommand.stdin, commandString)
	if err != nil {
		// Log the error
		log.Println(err)
		// Return a blank string and the error object
		return "", err
	}
	// Wait for the response text
	responseText = <-commandResponse

	// Return the response and a nil error to indicate success
	return responseText, nil
}

func getGasStatus() {
	text, err := sendCommand("gas info")
	if err != nil {
		log.Println(err)
		return
	}
	populateGasData(text)
}

func getTdsStatus() {
	text, err := sendCommand("tds info")
	if err != nil {
		log.Println(err)
		return
	}
	populateTdsData(text)
}

//func getFuelCellStatus(device uint32) (status *FCM804) {
//	return canBus.fuelCell[device]
//}

//func getFuelCellStatus(device int16) (status *fuelCellStatus) {
//	SystemStatus.m.Lock()
//	defer SystemStatus.m.Unlock()
//
//	fcStatus := SystemStatus.FuelCells[device-1]
//	fcStatus.State = "Switched Off"
//	if (device < 1) || (device > systemConfig.NumFc) {
//		log.Panic("Invalid fuel cell in get status - ", device)
//	}
//	if ((device == 1) && !SystemStatus.Relays.FuelCell1Enable) || ((device == 2) && !SystemStatus.Relays.FuelCell2Enable) {
//		return fcStatus
//	}
//
//	strCommand := fmt.Sprintf("fc info %d", device-1)
//	text, err := sendCommand(strCommand)
//	if err != nil {
//		log.Println(err)
//		fcStatus.State = "(Error)"
//		return fcStatus
//	}
//	return populateFuelCellData(text, fcStatus)
//}

func getRelayStatus() bool {
	text, err := sendCommand("relay status")
	if err != nil {
		log.Println(err)
		return false
	}
	return populateRelayData(text)
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

func getErrorAKey(index int) string {
	errors := []string{
		"AnodeOverPressure",
		"AnodeUnderPressure",
		"Stack1OverCurrent",
		"Outlet1OverTemperature",
		"Stack1MinCellUndervoltage",
		"Inlet1OverTemperature",
		"SafetyObserverWatchdogTrip",
		"BoardOverTemperature",
		"SafetyObserverFanTrip",
		"ValveDefeatCheckFault",
		"Stack1UnderVoltage",
		"Stack1OverVoltage",
		"SafetyObserverMismatch",
		"Stack2MinCellUndervoltage",
		"SafetyObserverPressureTrip",
		"SafetyObserverBoardTxTrip",
		"Stack3MinCellUndervoltage",
		"SafetyObserverSoftwareTrip",
		"Fan2NoTacho",
		"Fan1NoTacho",
		"Fan3NoTacho",
		"Fan3ErrantSpeed",
		"Fan2ErrantSpeed",
		"Fan1ErrantSpeed",
		"Sib1Fault",
		"Sib2Fault",
		"Sib3Fault",
		"Inlet1TxSensorFault",
		"Outlet1TxSensorFault",
		"InvalidSerialNumber",
		"Dcdc1CurrentWhenDisabled",
		"Dcdc1OverCurrent"}

	return errors[index]
}

func getErrorBKey(index int) string {
	errors := []string{"AmbientOverTemperature",
		"Sib1CommsFault",
		"BoardTxSensorFault",
		"Sib2CommsFault",
		"LowLeakTestPressure",
		"Sib3CommsFault",
		"LouverOpenFault",
		"StateDependentUnexpectedCurrent1",
		"Dcdc2CurrentWhenDisabled",
		"Dcdc3CurrentWhenDisabled",
		"Dcdc2OverCurrent",
		"ReadConfigFault",
		"CorruptConfigFault",
		"ConfigValueRangeFault",
		"Stack1VoltageMismatch",
		"Dcdc3OverCurrent",
		"UnexpectedPurgeInhibit",
		"FuelOnNoVolts",
		"LeakDetected",
		"AirCheckFault",
		"AirCheckFaultShadow",
		"DenyStartUV",
		"StateDependentUnexpectedCurrent2",
		"StateDependentUnexpectedCurrent3",
		"Stack2UnderVoltage",
		"Stack3UnderVoltage",
		"Stack2OverVoltage",
		"Stack3OverVoltage",
		"Stack2OverCurrent",
		"Stack3OverCurrent"}

	return errors[index]
}

func getErrorCKey(index int) string {
	errors := []string{
		"Stack2VoltageMismatch",
		"Stack3VoltageMismatch",
		"Outlet2OverTemperature",
		"Outlet3OverTemperature",
		"Inlet2OverTemperature",
		"Inlet3OverTemperature",
		"Inlet2TxSensorFault",
		"Inlet3TxSensorFault",
		"Outlet2TxSensorFault",
		"Outlet3TxSensorFault",
		"FuelOn1LowMeanVoltage",
		"FuelOn2LowMeanVoltage",
		"FuelOn3LowMeanVoltage",
		"FuelOn1LowMinVoltage",
		"FuelOn2LowMinVoltage",
		"FuelOn3LowMinVoltage",
		"SoftwareTripShutdown",
		"SoftwareTripFault",
		"TurnAroundTimeWarning",
		"PurgeCheckShutdown",
		"OutputUnderVoltage",
		"OutputOverVoltage",
		"SafetyObserverVoltRailTrip",
		"SafetyObserverDiffPressureTrip",
		"PurgeMissedOnePxOpen",
		"PurgeMissedOnePxClose",
		"PurgeMissedOneIxOpen",
		"PurgeMissedOneIxSolSaver",
		"PurgeMissedOneIxClose",
		"InRangeFaultPx01",
		"NoisyInputPx01",
		"NoisyInputTx68"}

	return errors[index]
}

func getErrorDKey(index int) string {
	errors := []string{
		"NoisyInputDiffP",
		"ValveClosedPxRising",
		"DiffPSensorFault",
		"LossOfVentilation",
		"DiffPSensorHigh",
		"FanOverrun",
		"BlockedAirFlow",
		"WarningNoisyInputPx01",
		"WarningNoisyInputTx68",
		"WarningNoisyInputDiffP",
		"Dcdc1OutputFault",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		""}

	return errors[index]
}

func getAllFuelCellErrors(FlagA uint32, FlagB uint32, FlagC uint32, FlagD uint32) string {
	faultA := getFuelCellError('A', FlagA)
	faultB := getFuelCellError('B', FlagB)
	faultC := getFuelCellError('C', FlagC)
	faultD := getFuelCellError('D', FlagD)
	faults := ""
	if len(faultA) > 0 {
		faults = strings.Join(faultA, ";")
	}
	if len(faultB) > 0 {
		if len(faults) > 0 {
			faults += "\r\n"
		}
		faults += strings.Join(faultB, ";")
	}
	if len(faultC) > 0 {
		if len(faults) > 0 {
			faults += "\r\n"
		}
		faults += strings.Join(faultC, ";")
	}
	if len(faultD) > 0 {
		if len(faults) > 0 {
			faults += "\r\n"
		}
		faults += strings.Join(faultD, ";")
	}
	return faults
}

func getFuelCellError(faultFlag rune, FaultFlag uint32) []string {
	var errors []string

	if FaultFlag == 0 {
		return errors
	}
	//faultFlagValue, err := strconv.ParseUint(FaultFlag, 16, 32)
	//if err != nil {
	//	log.Println(err)
	//	return errors
	//}
	if FaultFlag == 0xffffffff {
		return errors
	}
	mask := uint32(0x80000000)
	for i := 0; i < 32; i++ {
		if (FaultFlag & mask) != 0 {
			switch faultFlag {
			case 'A':
				errors = append(errors, getErrorAKey(i))
			case 'B':
				errors = append(errors, getErrorBKey(i))
			case 'C':
				errors = append(errors, getErrorCKey(i))
			case 'D':
				errors = append(errors, getErrorDKey(i))
			default:
				return []string{"Invalid FaultFlag"}
			}
		}
		mask >>= 1
	}
	return errors
}

func buildToolTip(errors []string) string {
	if len(errors) > 0 {
		strError := `<span style="color:red">` + strings.Join(errors, "<br />") + "</span>"
		return strError
	}
	return "No Error"
}

func getFuelCellHtmlStatus(status *FCM804) (html string) {

	if !status.IsSwitchedOn() {
		return `<h3 style="text-align:center">Fuel Cell is switched OFF</h3>"`
	}
	html = fmt.Sprintf(`<table>
 <tr><td class="label">Serial Number</td><td>%s</td><td class="label">Version</td><td>%d.%d.%d</td></tr>
 <tr><td class="label">Output Power</td><td>%dW</td><td class="label">Output Volts</td><td>%0.2fV</td></tr>
 <tr><td class="label">Output Current</td><td>%0.2fA</td><td class="label">Anode Pressure</td><td>%0.2f Millibar</td></tr>
 <tr><td class="label">Inlet Temperature</td><td>%0.2f℃</td><td class="label">Outlet Temperature</td><td>%0.2f℃</td></tr>
 <tr><td class="label" colspan=2>State</td><td colspan=2>%s</td></tr>
 <tr><td class="label">Fault Flag A</td><td>%s</td><td class="label">Fault Flag B</td><td>%s</td></tr>
 <tr><td class="label">Fault Flag C</td><td>%s</td><td class="label">Fault Flag D</td><td>%s</td></tr>
</table>`, string(status.Serial[:]), status.Software.Version, status.Software.Major, status.Software.Minor,
		status.OutputPower, status.OutputVolts,
		status.OutputCurrent, status.AnodePressure, status.InletTemp, status.OutletTemp,
		status.GetState(), buildToolTip(getFuelCellError('A', status.FaultA)),
		buildToolTip(getFuelCellError('B', status.FaultB)),
		buildToolTip(getFuelCellError('C', status.FaultC)),
		buildToolTip(getFuelCellError('D', status.FaultD)))
	return html
}

//func getFuelCellHtmlStatus(status *fuelCellStatus) (html string) {
//
//	if status.State == "Switched Off" {
//		return `<h3 style="text-align:center">Fuel Cell is switched OFF</h3>"`
//	}
//	html = fmt.Sprintf(`<table>
//  <tr><td class="label">Serial Number</td><td>%s</td><td class="label">Version</td><td>%s</td></tr>
//  <tr><td class="label">Output Power</td><td>%0.2fW</td><td class="label">Output Volts</td><td>%0.2fV</td></tr>
//  <tr><td class="label">Output Current</td><td>%0.2fA</td><td class="label">Anode Pressure</td><td>%0.2f Millibar</td></tr>
//  <tr><td class="label">Inlet Temperature</td><td>%0.2f℃</td><td class="label">Outlet Temperature</td><td>%0.2f℃</td></tr>
//  <tr><td class="label" colspan=2>State</td><td colspan=2>%s</td></tr>
//  <tr><td class="label">Fault Flag A</td><td>%s</td><td class="label">Fault Flag B</td><td>%s</td></tr>
//  <tr><td class="label">Fault Flag C</td><td>%s</td><td class="label">Fault Flag D</td><td>%s</td></tr>
//</table>`, status.SerialNumber, status.Version, status.OutputPower, status.OutputVolt,
//		status.OutputCurrent, status.AnodePress, status.InletTemp, status.OutletTemp,
//		status.State, buildToolTip(getFuelCellError('A', status.FaultFlagA)),
//		buildToolTip(getFuelCellError('B', status.FaultFlagB)),
//		buildToolTip(getFuelCellError('C', status.FaultFlagC)),
//		buildToolTip(getFuelCellError('D', status.FaultFlagD)))
//	return html
//}

/**
getGasHtmlStatus : return the html rendering of the Gas status from the gasStatus object
*/
func getGasHtmlStatus() (html string) {

	html = fmt.Sprintf(`<table>
  <tr><td class="label">Fuel Cell Pressure</td><td>%0.2f bar</td><td class="label">Tank Pressure</td><td>%0.1f bar</td></tr>
</table>`, SystemStatus.Gas.FuelCellPressure, SystemStatus.Gas.TankPressure)
	return html
}

/**
Convert relay status to english text
*/
func booleanToHtmlClass(b bool) string {
	if b {
		return "RelayOn"
	} else {
		return "RelayOff"
	}
}

/**
getRelayHtmlStatus : return the html rendering of the relay status object
*/
func getRelayHtmlStatus() (html string) {
	return fmt.Sprintf(`<table><tr><th colspan=8>Relay Status</th></tr><tr>
<th class="%s">Electrolyser 1</th><th class="%s">Electrolyser 2</th>
<th class="%s">Gas to Fuel Cell</th><th class="%s">Fuel Cell 1 Enable</th><th class="%s">Fuel Cell 1 Run 1</th>
<th class="%s">Fuel Cell 2 Enable</th><th class="%s">Fuel Cell 2 Run</th><th class="%s">Drain</th></tr></table>`,
		booleanToHtmlClass(SystemStatus.Relays.Electrolyser1),
		booleanToHtmlClass(SystemStatus.Relays.Electrolyser2),
		booleanToHtmlClass(SystemStatus.Relays.GasToFuelCell),
		booleanToHtmlClass(SystemStatus.Relays.FuelCell1Enable),
		booleanToHtmlClass(SystemStatus.Relays.FuelCell1Run),
		booleanToHtmlClass(SystemStatus.Relays.FuelCell2Enable),
		booleanToHtmlClass(SystemStatus.Relays.FuelCell2Run),
		booleanToHtmlClass(SystemStatus.Relays.Drain))
}

/**
getTdsHtmlStatus : return the html rendering of the Gas status from the gasStatus object
*/
func getTdsHtmlStatus() (html string) {

	html = fmt.Sprintf(`<table>
  <tr><td class="label">Total Dissolved Solids</td><td>%d ppm</td></tr>
</table>`, SystemStatus.TDS.TdsReading)
	return html
}

/**
getStatus : return tha status page showing the complete system status
*/
func getStatus(w http.ResponseWriter, _ *http.Request) {
	var jErr JSONError

	if !getRelayStatus() {
		jErr.AddErrorString("status", `<head><title>Firefly Status Error</title></head>
<body><h1>ERROR feching relay status.</h1><br />
<h3>One or more relays could not be identified in the "relay status" command.</h3>
</body></html>`)
		jErr.ReturnError(w, 500)
		return
	}

	if _, err := fmt.Fprintf(w, `<html>
  <head>
    <title>Firefly Status</title>
    <style>
td.label {
	text-align:right;
	border-right-style:solid;
	border-right-width:2px;
	padding-right:5px;
}
td {
	padding-left:5px;
}
div {
	padding:5px;
}
h2 {
	text-align:center;
}
table {
	margin:auto;
}
th.RelayOn {
	background-color:green;
	color:white;
	padding:10px
}
th.RelayOff {
	background-color:red;
	color:white;
	padding:10px
}
    </style>
  </head>
  <body>
	<div>
	  %s
	</div>`, getRelayHtmlStatus()); err != nil {
		jErr.AddError("relayStatus", err)
		jErr.ReturnError(w, 500)
		return
	}
	for idx, el := range SystemStatus.Electrolysers {
		if _, err := fmt.Fprintf(w, `<div><h2>Electrolyser %d</h2>%s</div>`, idx, getElectrolyserHtmlStatus(el)); err != nil {
			jErr.AddError("electrolyserStatus", err)
			jErr.ReturnError(w, 500)
			return
		}
	}
	if _, err := fmt.Fprintf(w, `<div><div style="float:left; width:48%%"><h2>Dryer</h2>%s</div>`, getDryerHtmlStatus(SystemStatus.Electrolysers[0])); err != nil {
		jErr.AddError("dryerStatus", err)
		jErr.ReturnError(w, 500)
		return
	}
	for idx, fc := range canBus.fuelCell {
		if _, err := fmt.Fprintf(w, `<div style="float:left; width:48%%"><h2>Fuel Cell %d</h2>%s<br /><a href="/fc/%d/restart">Restart</a></div>`, idx, getFuelCellHtmlStatus(fc), idx+1); err != nil {
			jErr.AddError("fuelCellStatus", err)
			jErr.ReturnError(w, 500)
			return
		}
	}
	if _, err := fmt.Fprintf(w, `<div style="float:left; clear:both; width:48%%"><h2>Gas</h2>%s</div><div style="float:left; width:48%%">
        <h2>TDS</h2>%s</div></div><div style="clear:both">
      <a href="/">Back to the Menu</a>
    </div>
  </body>
<script>
	var tID = setTimeout(function () {
		window.location.reload(true);
		window.clearTimeout(tID);		// clear time out.
	}, 5000);
</script>
</html>`, getGasHtmlStatus(), getTdsHtmlStatus()); err != nil {
		jErr.AddError("gasAndTDSStatus", err)
		jErr.ReturnError(w, 500)
	}
}

/**
Convert an error object to JSON to return to the caller
*/
func errorToJson(err error) string {
	var errStruct struct {
		Error string
	}
	errStruct.Error = err.Error()

	byteArray, errMarshal := json.Marshal(errStruct)
	if errMarshal != nil {
		log.Print(errMarshal)
	}
	return string(byteArray)
}

/**
Get the electrolyser status as a JSON object
*/
func getElectrolyserJsonStatus(w http.ResponseWriter, r *http.Request) {
	// Set the returned type to application/json
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 2) || (device < 1) {
		log.Print("Invalid electrolyser in status request")
		getStatus(w, r)
		return
	}
	bytesArray, err := json.Marshal(SystemStatus.Electrolysers[device-1].status)
	if err != nil {
		log.Println(SystemStatus.Electrolysers[device-1].status)
		if _, err := fmt.Fprint(w, errorToJson(err)); err != nil {
			log.Print(err)
		}
	}
	if _, err := fmt.Fprint(w, string(bytesArray)); err != nil {
		log.Print(err)
	}
}

/**
Get the dryer status as a JSON object
*/
func getDryerJsonStatus(w http.ResponseWriter, r *http.Request) {
	// Set the returned type to application/json
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 2) || (device < 1) {
		log.Print("Invalid dryer in status request")
		getStatus(w, r)
		return
	}
	bytesArray, err := json.Marshal(SystemStatus.Electrolysers[device].status)
	if err != nil {
		if _, err := fmt.Fprint(w, errorToJson(err)); err != nil {
			log.Print(err)
		}
	}
	if _, err := fmt.Fprint(w, string(bytesArray)); err != nil {
		log.Print(err)
	}
}

/**
Get the fuel cell status as a JSON object
*/
func getFuelCellJsonStatus(w http.ResponseWriter, r *http.Request) {
	// Set the returned type to application/json
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 2) || (device < 1) {
		log.Print("Invalid fuel cell in status request")
		getStatus(w, r)
		return
	}
	SystemStatus.m.Lock()
	defer SystemStatus.m.Unlock()

	//	var status *fuelCellStatus
	//	if (device == 1) && SystemStatus.Relays.FuelCell1Enable {
	//		status = getFuelCellStatus(int16(device))
	//	} else {
	//		status = new(fuelCellStatus)
	//		status.State = "Switched Off"
	//	}
	bytesArray, err := json.Marshal(canBus.fuelCell[uint16(device)])
	if err != nil {
		if _, err := fmt.Fprint(w, errorToJson(err)); err != nil {
			log.Print(err)
		}
	}
	if _, err := fmt.Fprint(w, string(bytesArray)); err != nil {
		log.Print(err)
	}
}

// This function will set the electrolyser status to switched on. It can be scheduled using the timeAfterFunc method
func enableDevice(device int) func() {
	return func() {
		SystemStatus.Electrolysers[device].status.SwitchedOn = true
	}
}

/**
Return the current system status
*/
func getSystemStatus() {
	if !getRelayStatus() {
		SystemStatus.valid = false
		return
	}
	SystemStatus.m.Lock()
	defer SystemStatus.m.Unlock()

	getGasStatus()
	getTdsStatus()
	// Moved to a separate logging loop
	//	for device := range SystemStatus.FuelCells {
	//		getFuelCellStatus(int16(device + 1))
	//	}

	for device := range SystemStatus.Electrolysers {

		ElectrolyserOn := false
		// Check the power relay to see if this electrolyser has power
		switch device {
		case 0:
			ElectrolyserOn = SystemStatus.Relays.Electrolyser1
		case 1:
			ElectrolyserOn = SystemStatus.Relays.Electrolyser2
		default:
			log.Println("invalid electrolyser in getSystemStatus")
		}
		// If the relay is closed but the status still shows that the electrolyser is off we should give it 5 seconds to power up before we try to get its status
		// otherwise we will get a load of errors that are not real.
		if (!SystemStatus.Electrolysers[device].status.SwitchedOn) && ElectrolyserOn {
			// Get the function to set the status. This is necessary becuase we cannot directly pass parameters to the timeAfterFunc function
			f := enableDevice(device)
			// Schedule this function to run in 5 seconds to give the Electrolyser time to power up
			time.AfterFunc(time.Second*5, f)
		}
		// If the electrolyser is showing on but the relay is off, immediately set the electrolyser status to powered off
		if !ElectrolyserOn {
			SystemStatus.Electrolysers[device].status.SwitchedOn = false
		}
		// If the electrolyser shows powered up, get the current status
		if SystemStatus.Electrolysers[device].status.SwitchedOn {
			SystemStatus.Electrolysers[device].ReadValues()
		}
	}
	SystemStatus.valid = true
}

//func pollFuelCells() {
//	for device := range SystemStatus.FuelCells {
//		if (device == 0 && SystemStatus.Relays.FuelCell1Enable) || (device == 1 && SystemStatus.Relays.FuelCell2Enable) {
//			getFuelCellStatus(uint32(device + 1))
//		}
//	}
//}

func logFuelCellData() {
	var data struct {
		Cell          sql.NullInt32
		AnodePressure sql.NullInt32
		FaultA        uint32
		FaultB        uint32
		FaultC        uint32
		FaultD        uint32
		InletTemp     sql.NullInt32
		OutletTemp    sql.NullInt32
		Power         sql.NullInt32
		Amps          sql.NullInt32
		Volts         sql.NullInt32
		State         sql.NullString
	}
	strCommand := `INSERT INTO firefly.FuelCell(AnodePressure, Power, FaultA, FaultB, FaultC, FaultD, OutletTemp, InletTemp, Volts, Amps, State, Cell)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`

	var err error
	if pDB == nil {
		if pDB, err = connectToDatabase(); err != nil {
			log.Print(err)
			return
		}
	}

	for device, fuelCell := range canBus.fuelCell {
		if !fuelCell.IsSwitchedOn() {
			data.Cell.Int32 = int32(device)
			data.Cell.Valid = true
			data.AnodePressure.Int32 = int32(fuelCell.AnodePressure * 1000)
			data.AnodePressure.Valid = true
			data.FaultA = fuelCell.FaultA
			data.FaultB = fuelCell.FaultB
			data.FaultC = fuelCell.FaultC
			data.FaultD = fuelCell.FaultD
			data.InletTemp.Int32 = int32(fuelCell.InletTemp * 10)
			data.InletTemp.Valid = true
			data.OutletTemp.Int32 = int32(fuelCell.OutletTemp * 10)
			data.OutletTemp.Valid = true
			data.Amps.Int32 = int32(fuelCell.OutputCurrent * 100)
			data.Amps.Valid = true
			data.Volts.Int32 = int32(fuelCell.OutputVolts * 10)
			data.Volts.Valid = true
			data.Power.Int32 = int32(fuelCell.OutputPower)
			data.Power.Valid = true
			data.State.String = fuelCell.GetState()
			data.State.Valid = true

			_, err = pDB.Exec(strCommand,
				data.AnodePressure, data.Power, data.FaultA, data.FaultB, data.FaultC, data.FaultD, data.OutletTemp, data.InletTemp,
				data.Volts, data.Amps, data.State, data.Cell)
			if err != nil {
				log.Printf("Error writing fuel cell values to the database - %s", err)
				_ = pDB.Close()
				pDB = nil
			}
		}
	}
}

/**
Log the current system status to the database
*/
func logStatus() {
	var err error
	if pDB == nil {
		if pDB, err = connectToDatabase(); err != nil {
			log.Print(err)
			return
		}
	}

	var params struct {
		el1Rate             sql.NullInt64
		el1ElectrolyteLevel sql.NullString
		el1ElectrolyteTemp  sql.NullFloat64
		el1State            sql.NullInt32
		el1H2Flow           sql.NullFloat64
		el1H2InnerPressure  sql.NullFloat64
		el1H2OuterPressure  sql.NullFloat64
		el1StackVoltage     sql.NullFloat64
		el1StackCurrent     sql.NullFloat64
		el1SystemState      sql.NullInt32
		el1WaterPressure    sql.NullFloat64

		dr1Temp0          sql.NullFloat64
		dr1Temp1          sql.NullFloat64
		dr1Temp2          sql.NullFloat64
		dr1Temp3          sql.NullFloat64
		dr1InputPressure  sql.NullFloat64
		dr1OutputPressure sql.NullFloat64
		dr1Warning        sql.NullString
		dr1Error          sql.NullString

		el2Rate             sql.NullInt64
		el2ElectrolyteLevel sql.NullString
		el2ElectrolyteTemp  sql.NullFloat64
		el2State            sql.NullInt32
		el2H2Flow           sql.NullFloat64
		el2H2InnerPressure  sql.NullFloat64
		el2H2OuterPressure  sql.NullFloat64
		el2StackVoltage     sql.NullFloat64
		el2StackCurrent     sql.NullFloat64
		el2SystemState      sql.NullInt32
		el2WaterPressure    sql.NullFloat64

		dr2Temp0          sql.NullFloat64
		dr2Temp1          sql.NullFloat64
		dr2Temp2          sql.NullFloat64
		dr2Temp3          sql.NullFloat64
		dr2InputPressure  sql.NullFloat64
		dr2OutputPressure sql.NullFloat64
		dr2Warning        sql.NullString
		dr2Error          sql.NullString

		fc1State         sql.NullString
		fc1AnodePressure sql.NullFloat64
		fc1FaultFlagA    uint32
		fc1FaultFlagB    uint32
		fc1FaultFlagC    uint32
		fc1FaultFlagD    uint32
		fc1InletTemp     sql.NullFloat64
		fc1OutletTemp    sql.NullFloat64
		fc1OutputPower   int16
		fc1OutputCurrent sql.NullFloat64
		fc1OutputVoltage sql.NullFloat64

		fc2State         sql.NullString
		fc2AnodePressure sql.NullFloat64
		fc2FaultFlagA    uint32
		fc2FaultFlagB    uint32
		fc2FaultFlagC    uint32
		fc2FaultFlagD    uint32
		fc2InletTemp     sql.NullFloat64
		fc2OutletTemp    sql.NullFloat64
		fc2OutputPower   int16
		fc2OutputCurrent sql.NullFloat64
		fc2OutputVoltage sql.NullFloat64
	}

	SystemStatus.m.Lock()
	defer SystemStatus.m.Unlock()

	params.el1Rate.Valid = false
	params.el1ElectrolyteLevel.Valid = false
	params.el1ElectrolyteTemp.Valid = false
	params.el1State.Valid = false
	params.el1H2Flow.Valid = false
	params.el1H2InnerPressure.Valid = false
	params.el1H2OuterPressure.Valid = false
	params.el1StackVoltage.Valid = false
	params.el1StackCurrent.Valid = false
	params.el1SystemState.Valid = false
	params.el1WaterPressure.Valid = false

	params.el2Rate.Valid = false
	params.el2ElectrolyteLevel.Valid = false
	params.el2ElectrolyteTemp.Valid = false
	params.el2State.Valid = false
	params.el2H2Flow.Valid = false
	params.el2H2InnerPressure.Valid = false
	params.el2H2OuterPressure.Valid = false
	params.el2StackVoltage.Valid = false
	params.el2StackCurrent.Valid = false
	params.el2SystemState.Valid = false
	params.el2WaterPressure.Valid = false

	params.dr1Temp0.Valid = false
	params.dr1Temp1.Valid = false
	params.dr1Temp2.Valid = false
	params.dr1Temp3.Valid = false
	params.dr1InputPressure.Valid = false
	params.dr1OutputPressure.Valid = false
	params.dr1Warning.Valid = false
	params.dr1Error.Valid = false

	params.dr2Temp0.Valid = false
	params.dr2Temp1.Valid = false
	params.dr2Temp2.Valid = false
	params.dr2Temp3.Valid = false
	params.dr2InputPressure.Valid = false
	params.dr2OutputPressure.Valid = false
	params.dr2Warning.Valid = false
	params.dr2Error.Valid = false

	params.fc1State.Valid = false
	params.fc1AnodePressure.Valid = false
	params.fc1InletTemp.Valid = false
	params.fc1OutletTemp.Valid = false
	params.fc1OutputCurrent.Valid = false
	params.fc1OutputVoltage.Valid = false

	params.fc2State.Valid = false
	params.fc2AnodePressure.Valid = false
	params.fc2InletTemp.Valid = false
	params.fc2OutletTemp.Valid = false
	params.fc2OutputCurrent.Valid = false
	params.fc2OutputVoltage.Valid = false

	if len(SystemStatus.Electrolysers) > 0 {
		if SystemStatus.Relays.Electrolyser1 {
			params.el1SystemState.Int32 = int32(SystemStatus.Electrolysers[0].status.SystemState)
			params.el1SystemState.Valid = true
			params.el1ElectrolyteLevel.String = SystemStatus.Electrolysers[0].status.ElectrolyteLevel.String()
			params.el1ElectrolyteLevel.Valid = true
			params.el1H2Flow.Float64 = float64(SystemStatus.Electrolysers[0].status.H2Flow)
			params.el1H2Flow.Valid = true
			params.el1ElectrolyteTemp.Float64 = float64(SystemStatus.Electrolysers[0].status.ElectrolyteTemp)
			params.el1ElectrolyteTemp.Valid = true
			params.el1State.Int32 = int32(SystemStatus.Electrolysers[0].status.ElState)
			params.el1State.Valid = true
			params.el1H2InnerPressure.Float64 = float64(SystemStatus.Electrolysers[0].status.InnerH2Pressure)
			params.el1H2InnerPressure.Valid = true
			params.el1H2OuterPressure.Float64 = float64(SystemStatus.Electrolysers[0].status.OuterH2Pressure)
			params.el1H2OuterPressure.Valid = true
			params.el1Rate.Int64 = int64(SystemStatus.Electrolysers[0].GetRate())
			params.el1Rate.Valid = true
			params.el1StackVoltage.Float64 = float64(SystemStatus.Electrolysers[0].status.StackVoltage)
			params.el1StackVoltage.Valid = true
			params.el1StackCurrent.Float64 = float64(SystemStatus.Electrolysers[0].status.StackCurrent)
			params.el1StackCurrent.Valid = true
			params.el1WaterPressure.Float64 = float64(SystemStatus.Electrolysers[0].status.WaterPressure)
			params.el1WaterPressure.Valid = true
			params.dr1InputPressure.Float64 = float64(SystemStatus.Electrolysers[0].status.DryerInputPressure)
			params.dr1InputPressure.Valid = true
			params.dr1OutputPressure.Float64 = float64(SystemStatus.Electrolysers[0].status.DryerOutputPressure)
			params.dr1OutputPressure.Valid = true
			params.dr1Temp0.Float64 = float64(SystemStatus.Electrolysers[0].status.DryerTemp1)
			params.dr1Temp0.Valid = true
			params.dr1Temp1.Float64 = float64(SystemStatus.Electrolysers[0].status.DryerTemp2)
			params.dr1Temp1.Valid = true
			params.dr1Temp2.Float64 = float64(SystemStatus.Electrolysers[0].status.DryerTemp3)
			params.dr1Temp2.Valid = true
			params.dr1Temp3.Float64 = float64(SystemStatus.Electrolysers[0].status.DryerTemp4)
			params.dr1Temp3.Valid = true
			params.dr1Warning.String = SystemStatus.Electrolysers[0].GetDryerWarningText()
			params.dr1Warning.Valid = true
			params.dr1Error.String = SystemStatus.Electrolysers[0].GetDryerErrorText()
			params.dr1Error.Valid = true

		} else {
			params.el1SystemState.Int32 = -1
			params.el1SystemState.Valid = true
		}
	}
	if len(SystemStatus.Electrolysers) > 1 {
		if SystemStatus.Relays.Electrolyser2 {
			params.el2SystemState.Int32 = int32(SystemStatus.Electrolysers[1].status.SystemState)
			params.el2SystemState.Valid = true
			params.el2ElectrolyteLevel.String = SystemStatus.Electrolysers[1].status.ElectrolyteLevel.String()
			params.el2ElectrolyteLevel.Valid = true
			params.el2H2Flow.Float64 = float64(SystemStatus.Electrolysers[1].status.H2Flow)
			params.el2H2Flow.Valid = true
			params.el2ElectrolyteTemp.Float64 = float64(SystemStatus.Electrolysers[1].status.ElectrolyteTemp)
			params.el2ElectrolyteTemp.Valid = true
			params.el2State.Int32 = int32(SystemStatus.Electrolysers[1].status.ElState)
			params.el2State.Valid = true
			params.el2H2InnerPressure.Float64 = float64(SystemStatus.Electrolysers[1].status.InnerH2Pressure)
			params.el2H2InnerPressure.Valid = true
			params.el2H2OuterPressure.Float64 = float64(SystemStatus.Electrolysers[1].status.OuterH2Pressure)
			params.el2H2OuterPressure.Valid = true
			params.el2Rate.Int64 = int64(SystemStatus.Electrolysers[1].GetRate())
			params.el2Rate.Valid = true
			params.el2StackVoltage.Float64 = float64(SystemStatus.Electrolysers[1].status.StackVoltage)
			params.el2StackVoltage.Valid = true
			params.el2StackCurrent.Float64 = float64(SystemStatus.Electrolysers[1].status.StackCurrent)
			params.el2StackCurrent.Valid = true
			params.el2WaterPressure.Float64 = float64(SystemStatus.Electrolysers[1].status.WaterPressure)
			params.el2WaterPressure.Valid = true
		} else {
			params.el2SystemState.Int32 = -1
			params.el2SystemState.Valid = true
		}
	}
	if len(SystemStatus.Electrolysers) > 0 {
		if SystemStatus.Relays.Electrolyser1 {
			params.dr1InputPressure.Float64 = float64(SystemStatus.Electrolysers[0].status.DryerInputPressure)
			params.dr1InputPressure.Valid = true
			params.dr1OutputPressure.Float64 = float64(SystemStatus.Electrolysers[0].status.DryerOutputPressure)
			params.dr1OutputPressure.Valid = true
			params.dr1Temp0.Float64 = float64(SystemStatus.Electrolysers[0].status.DryerTemp1)
			params.dr1Temp0.Valid = true
			params.dr1Temp1.Float64 = float64(SystemStatus.Electrolysers[0].status.DryerTemp2)
			params.dr1Temp1.Valid = true
			params.dr1Temp2.Float64 = float64(SystemStatus.Electrolysers[0].status.DryerTemp3)
			params.dr1Temp2.Valid = true
			params.dr1Temp3.Float64 = float64(SystemStatus.Electrolysers[0].status.DryerTemp4)
			params.dr1Temp3.Valid = true
			params.dr1Warning.String = SystemStatus.Electrolysers[0].GetDryerWarningText()
			params.dr1Warning.Valid = true
			params.dr1Error.String = SystemStatus.Electrolysers[0].GetDryerErrorText()
			params.dr1Error.Valid = true
		}
	}
	if len(SystemStatus.Electrolysers) > 1 {
		if SystemStatus.Relays.Electrolyser2 {
			params.dr2InputPressure.Float64 = float64(SystemStatus.Electrolysers[1].status.DryerInputPressure)
			params.dr2InputPressure.Valid = true
			params.dr2OutputPressure.Float64 = float64(SystemStatus.Electrolysers[1].status.DryerOutputPressure)
			params.dr2OutputPressure.Valid = true
			params.dr2Temp0.Float64 = float64(SystemStatus.Electrolysers[1].status.DryerTemp1)
			params.dr2Temp0.Valid = true
			params.dr2Temp1.Float64 = float64(SystemStatus.Electrolysers[1].status.DryerTemp2)
			params.dr2Temp1.Valid = true
			params.dr2Temp2.Float64 = float64(SystemStatus.Electrolysers[1].status.DryerTemp3)
			params.dr2Temp2.Valid = true
			params.dr2Temp3.Float64 = float64(SystemStatus.Electrolysers[1].status.DryerTemp4)
			params.dr2Temp3.Valid = true
			params.dr2Warning.String = SystemStatus.Electrolysers[1].GetDryerWarningText()
			params.dr2Warning.Valid = true
			params.dr2Error.String = SystemStatus.Electrolysers[1].GetDryerErrorText()
			params.dr2Error.Valid = true
		}
	}
	if fc, found := canBus.fuelCell[0]; found {
		if SystemStatus.Relays.FuelCell1Enable {
			params.fc1AnodePressure.Float64 = float64(fc.AnodePressure)
			params.fc1AnodePressure.Valid = true
			params.fc1FaultFlagA = fc.FaultA
			params.fc1FaultFlagB = fc.FaultB
			params.fc1FaultFlagC = fc.FaultC
			params.fc1FaultFlagD = fc.FaultD
			params.fc1InletTemp.Float64 = float64(fc.InletTemp)
			params.fc1InletTemp.Valid = true
			params.fc1OutletTemp.Float64 = float64(fc.OutletTemp)
			params.fc1OutletTemp.Valid = true
			params.fc1OutputCurrent.Float64 = float64(fc.OutputCurrent)
			params.fc1OutputCurrent.Valid = true
			params.fc1OutputVoltage.Float64 = float64(fc.OutputVolts)
			params.fc1OutputVoltage.Valid = true
			params.fc1OutputPower = fc.OutputPower
			params.fc1State.String = fc.GetState()
			params.fc1State.Valid = true
		} else {
			params.fc1State.String = "Powered Down"
			params.fc1State.Valid = true
		}
	}
	if fc, found := canBus.fuelCell[1]; found {
		if SystemStatus.Relays.FuelCell2Enable {
			params.fc2AnodePressure.Float64 = float64(fc.AnodePressure)
			params.fc2AnodePressure.Valid = true
			params.fc2FaultFlagA = fc.FaultA
			params.fc2FaultFlagB = fc.FaultB
			params.fc2FaultFlagC = fc.FaultC
			params.fc2FaultFlagD = fc.FaultD
			params.fc2InletTemp.Float64 = float64(fc.InletTemp)
			params.fc2InletTemp.Valid = true
			params.fc2OutletTemp.Float64 = float64(fc.OutletTemp)
			params.fc2OutletTemp.Valid = true
			params.fc2OutputCurrent.Float64 = float64(fc.OutputCurrent)
			params.fc2OutputCurrent.Valid = true
			params.fc2OutputVoltage.Float64 = float64(fc.OutputVolts)
			params.fc2OutputVoltage.Valid = true
			params.fc1OutputPower = fc.OutputPower
			params.fc2State.String = fc.GetState()
			params.fc2State.Valid = true
		} else {
			params.fc2State.String = "Powered Down"
			params.fc2State.Valid = true
		}
	}

	strCommand := `INSERT INTO firefly.logging(
            el1Rate, el1ElectrolyteLevel, el1ElectrolyteTemp, el1StateCode, el1H2Flow, el1H2InnerPressure, el1H2OuterPressure, el1StackVoltage, el1StackCurrent, el1SystemStateCode, el1WaterPressure, 
            dr1Temp0, dr1Temp1, dr1Temp2, dr1Temp3, dr1InputPressure, dr1OutputPressure, dr1Warning, dr1Error, 
            el2Rate, el2ElectrolyteLevel, el2ElectrolyteTemp, el2StateCode, el2H2Flow, el2H2InnerPressure, el2H2OuterPressure, el2StackVoltage, el2StackCurrent, el2SystemStateCode, el2WaterPressure,
            dr2Temp0, dr2Temp1, dr2Temp2, dr2Temp3, dr2InputPressure, dr2OutputPressure, dr2Warning, dr2Error,
            fc1State, fc1AnodePressure, fc1FaultFlagA, fc1FaultFlagB, fc1FaultFlagC, fc1FaultFlagD, fc1InletTemp, fc1OutletTemp, fc1OutputPower, fc1OutputCurrent, fc1OutputVoltage,
            fc2State, fc2AnodePressure, fc2FaultFlagA, fc2FaultFlagB, fc2FaultFlagC, fc2FaultFlagD, fc2InletTemp, fc2OutletTemp, fc2OutputPower, fc2OutputCurrent, fc2OutputVoltage,
            gasFuelCellPressure, gasTankPressure,
            totalDissolvedSolids,
            relayGas, relayFuelCell1Enable, relayFuelCell1Run, relayFuelCell2Enable, relayFuelCell2Run, relayEl1Power, relayEl2Power, relayDrain)
	VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
	       ?, ?, ?, ?, ?, ?, ?, ?,
	       ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
	       ?, ?, ?, ?, ?, ?, ?, ?,
	       ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
	       ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
	       ?, ?,
	       ?,
	       ?, ?, ?, ?, ?, ?, ?, ?);`

	_, err = pDB.Exec(strCommand,
		params.el1Rate, params.el1ElectrolyteLevel, params.el1ElectrolyteTemp, params.el1State, params.el1H2Flow, params.el1H2InnerPressure, params.el1H2OuterPressure, params.el1StackVoltage, params.el1StackCurrent, params.el1SystemState, params.el1WaterPressure,
		params.dr1Temp0, params.dr1Temp1, params.dr1Temp2, params.dr1Temp3, params.dr1InputPressure, params.dr1OutputPressure, params.dr1Warning, params.dr1Error,
		params.el2Rate, params.el2ElectrolyteLevel, params.el2ElectrolyteTemp, params.el2State, params.el2H2Flow, params.el2H2InnerPressure, params.el2H2OuterPressure, params.el2StackVoltage, params.el2StackCurrent, params.el2SystemState, params.el2WaterPressure,
		params.dr2Temp0, params.dr2Temp1, params.dr2Temp2, params.dr2Temp3, params.dr2InputPressure, params.dr2OutputPressure, params.dr2Warning, params.dr2Error,
		params.fc1State, params.fc1AnodePressure, params.fc1FaultFlagA, params.fc1FaultFlagB, params.fc1FaultFlagC, params.fc1FaultFlagD, params.fc1InletTemp, params.fc1OutletTemp, params.fc1OutputPower, params.fc1OutputCurrent, params.fc1OutputVoltage,
		params.fc2State, params.fc2AnodePressure, params.fc2FaultFlagA, params.fc2FaultFlagB, params.fc2FaultFlagC, params.fc2FaultFlagD, params.fc2InletTemp, params.fc2OutletTemp, params.fc2OutputPower, params.fc2OutputCurrent, params.fc2OutputVoltage,
		SystemStatus.Gas.FuelCellPressure, SystemStatus.Gas.TankPressure,
		SystemStatus.TDS.TdsReading,
		SystemStatus.Relays.GasToFuelCell, SystemStatus.Relays.FuelCell1Enable, SystemStatus.Relays.FuelCell1Run, SystemStatus.Relays.FuelCell2Enable, SystemStatus.Relays.FuelCell2Run, SystemStatus.Relays.Electrolyser1, SystemStatus.Relays.Electrolyser2, SystemStatus.Relays.Drain)

	if err != nil {
		log.Printf("Error writing inverter values to the database - %s", err)
		_ = pDB.Close()
		pDB = nil
	}
}

/**
Get running status as a JSON object
*/
func getMinJsonStatus() string {
	SystemStatus.m.Lock()
	defer SystemStatus.m.Unlock()

	type minElectrolyserStatus struct {
		On    bool
		State string
		Rate  int8
		Flow  float32
	}
	type minFuelCellStatus struct {
		On             bool       `json:"On"`
		State          string     `json:"State"`
		Output         float32    `json:"Output"`
		Alarm          string     `json:"Alarm"`
		FaultLevel     FaultLevel `json:"FaultLevel"`
		RebootRequired bool       `json:"RebootRequired"`
	}
	var minStatus struct {
		Electrolysers []*minElectrolyserStatus
		FuelCells     []*minFuelCellStatus
		Gas           float32
	}
	minStatus.Gas = float32(SystemStatus.Gas.TankPressure)
	for elnum, el := range SystemStatus.Electrolysers {
		minEl := new(minElectrolyserStatus)
		if elnum == 0 {
			minEl.On = SystemStatus.Relays.Electrolyser1
		} else {
			minEl.On = SystemStatus.Relays.Electrolyser2
		}
		if minEl.On {
			minEl.Rate = int8(el.GetRate())
			minEl.State = el.getState()
			minEl.Flow = float32(el.status.H2Flow)
		}
		minStatus.Electrolysers = append(minStatus.Electrolysers, minEl)
	}
	for _, fc := range canBus.fuelCell {
		minFc := new(minFuelCellStatus)
		minFc.State = fc.GetState()
		minFc.Output = float32(fc.OutputPower)
		minFc.Alarm = getAllFuelCellErrors(fc.FaultA, fc.FaultB, fc.FaultC, fc.FaultD)
		minFc.FaultLevel, minFc.RebootRequired = fc.GetFaultLevel()
		minStatus.FuelCells = append(minStatus.FuelCells, minFc)
	}

	if status, err := json.Marshal(minStatus); err != nil {
		log.Println("Error marshalling minStatus to JSON - ", err)
		return ""
	} else {
		return string(status)
	}
}

func getMinHtmlStatus(w http.ResponseWriter, _ *http.Request) {
	// Set the returned type to application/json
	w.Header().Set("Content-Type", "application/json")

	if _, err := fmt.Fprint(w, getMinJsonStatus()); err != nil {
		log.Println("Error getting MinHtmlStatus - ", err)
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
		var jErr JSONError
		jErr.AddError("database", err)
		jErr.ReturnError(w, 500)
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
Get fuel cell recorded values
*/
func getFuelCellHistory(w http.ResponseWriter, r *http.Request) {
	type Row struct {
		Logged     string
		FC1Volts   float64
		FC1Current float64
		FC1Power   float64
		FC2Volts   float64
		FC2Current float64
		FC2Power   float64
		H2Pressure float64
	}

	var results []*Row
	// Set the returned type to application/json
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	from := vars["from"]
	to := vars["to"]

	rows, err := pDB.Query(`SELECT (UNIX_TIMESTAMP(logged) DIV 60) * 60, IFNULL(AVG(fc1OutputVoltage),0), IFNULL(AVG(fc1OutputCurrent),0), IFNULL(AVG(fc1OutputPower),0), 
       IFNULL(AVG(fc2OutputVoltage),0), IFNULL(AVG(fc2OutputCurrent),0), IFNULL(AVG(fc2OutputPower),0), AVG(gasTankPressure)
  FROM firefly.logging
  WHERE logged BETWEEN ? and ?
  GROUP BY UNIX_TIMESTAMP(logged) DIV 60`, from, to)
	if err != nil {
		if _, err := fmt.Fprintf(w, `{"error":"%s"}`, err.Error()); err != nil {
			log.Println(err)
		}
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Println("Error closing query - ", err)
		}
	}()
	for rows.Next() {
		row := new(Row)
		if err := rows.Scan(&(row.Logged), &(row.FC1Volts), &(row.FC1Current), &(row.FC1Power),
			&(row.FC2Volts), &(row.FC2Current), &(row.FC2Power), &(row.H2Pressure)); err != nil {
			log.Print(err)
		} else {
			results = append(results, row)
		}
	}
	if JSON, err := json.Marshal(results); err != nil {
		if _, err := fmt.Fprintf(w, `{"error":"%s"`, err.Error()); err != nil {
			log.Println(err)
		}
	} else {
		if _, err := fmt.Fprintf(w, string(JSON)); err != nil {
			log.Println(err)
		}
	}
}

/**
Turn the given electrolyser on
*/
func setElOn(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device := vars["device"]
	var strCommand string
	deviceNum, err := strconv.ParseInt(device, 10, 8)
	if err != nil {
		log.Println("Failed to get the device. - ", err)
	}
	if err := validateDevice(uint8(deviceNum) - 1); err != nil {
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
		var jErr JSONError
		log.Print(err)
		jErr.AddError("Electrolyser", err)
		jErr.ReturnError(w, 500)
		return
	}
	_, err = fmt.Fprintf(w, `{"success":true}`)
	if err != nil {
		log.Print(err)
	}
	//	log.Println("Electrolyser turned on.")
}

/**
Turn all electrolysers on
*/
func setAllElOn(w http.ResponseWriter, _ *http.Request) {
	_, err := sendCommand("el1dr on")
	if err != nil {
		var jErr JSONError
		log.Print(err)
		jErr.AddError("Electrolyser", err)
		jErr.ReturnError(w, 500)
		return
	}
	_, err = sendCommand("el2 on")
	if err != nil {
		var jErr JSONError
		log.Print(err)
		jErr.AddError("Electrolyser", err)
		jErr.ReturnError(w, 500)
		return
	}
	_, err = fmt.Fprintf(w, `{"success":true}`)
	if err != nil {
		log.Print(err)
	}
}

/**
Turn the given electrolyser off
*/
func setElOff(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device := vars["device"]
	var strCommand string
	switch device {
	case "1":
		strCommand = "el1dr off"
		if SystemStatus.Electrolysers[0].status.StackVoltage > 30 {
			log.Println("Electrolyser 1 not turned off because stack voltage is too high.")
			var jErr JSONError
			jErr.AddErrorString("Electrolyser", "Electrolyser 1 not turned off because stack voltage is too high.")
			jErr.ReturnError(w, 400)
			return
		}
	case "2":
		strCommand = "el2 off"
		if SystemStatus.Electrolysers[1].status.StackVoltage > 30 {
			log.Println("Electrolyser 2 not turned off because stack voltage is too high.")
			var jErr JSONError
			jErr.AddErrorString("Electrolyser", "Electrolyser 2 not turned off because stack voltage is too high.")
			jErr.ReturnError(w, 400)
			return
		}
	default:
		log.Print("Invalid electrolyser specified - ", device)
		getStatus(w, r)
		return
	}
	_, err := sendCommand(strCommand)
	if err != nil {
		var jErr JSONError
		log.Print(err)
		jErr.AddError("Electrolyser", err)
		jErr.ReturnError(w, 500)
		return
	}
	_, err = fmt.Fprintf(w, `{"success":true}`)
	if err != nil {
		log.Print(err)
	}
}

/**
Turn all electrolysers off
*/
func setAllElOff(w http.ResponseWriter, _ *http.Request) {
	//	log.Println("Setting all electrolysers off")
	_, err := sendCommand("el2 off")
	if err != nil {
		var jErr JSONError
		log.Print(err)
		jErr.AddError("Electrolyser", err)
		jErr.ReturnError(w, 500)
		return
	}
	_, err = sendCommand("el1dr off")
	if err != nil {
		var jErr JSONError
		log.Print(err)
		jErr.AddError("Electrolyser", err)
		jErr.ReturnError(w, 500)
		return
	}
	_, err = fmt.Fprintf(w, `{"success":true}`)
	if err != nil {
		log.Print(err)
	}
}

/**
Turn the fuel cell gas on
*/
func setGasOn(w http.ResponseWriter, _ *http.Request) {
	response, err := sendCommand("gas on")
	if err != nil {
		log.Print(err)
		_, err = fmt.Fprintf(w, err.Error())
		if err != nil {
			log.Print(err)
		}
		return
	}
	_, err = fmt.Fprintf(w, `<html>
  <head>
    <title>Firefly Gas On</title>
  </head>
  <body>
    <div>%s</div>
	<div>
      <h2>You will be redirected to the status page in a moment.</h2>
    </div>
  </body>%s
</html>`, response, redirectToMainMenuScript)
	if err != nil {
		log.Print(err)
	}
}

/**
Turn the fuel cell gas off
*/
func setGasOff(w http.ResponseWriter, _ *http.Request) {
	response, err := sendCommand("gas off")
	if err != nil {
		log.Print(err)
		_, err = fmt.Fprintf(w, err.Error())
		if err != nil {
			log.Print(err)
		}
		return
	}
	_, err = fmt.Fprintf(w, `<html>
  <head>
    <title>Firefly Gas Off</title>
  </head>
  <body>
    <div>%s</div>
	<div>
      <h2>You will be redirected to the status page in a moment.</h2>
    </div>
  </body>%s
</html>`, response, redirectToMainMenuScript)
	if err != nil {
		log.Print(err)
	}
}

/**
Turn on the drain solenoid
*/
func setDrainOn(w http.ResponseWriter, _ *http.Request) {
	response, err := sendCommand("drain on")
	if err != nil {
		log.Print(err)
		_, err = fmt.Fprintf(w, err.Error())
		if err != nil {
			log.Print(err)
		}
		return
	}
	_, err = fmt.Fprintf(w, `<html>
  <head>
    <title>Firefly Drain On</title>
  </head>
  <body>
    <div>%s</div>
	<div>
      <h2>You will be redirected to the status page in a moment.</h2>
    </div>
  </body>%s
</html>`, response, redirectToMainMenuScript)
	if err != nil {
		log.Print(err)
	}
}

/**
Turn off the drain solenoid
*/
func setDrainOff(w http.ResponseWriter, _ *http.Request) {
	response, err := sendCommand("drain off")
	if err != nil {
		log.Print(err)
		_, err = fmt.Fprintf(w, err.Error())
		if err != nil {
			log.Print(err)
		}
		return
	}
	_, err = fmt.Fprintf(w, `<html>
  <head>
    <title>Firefly Drain Off</title>
  </head>
  <body>
    <div>%s</div>
	<div>
      <h2>You will be redirected to the status page in a moment.</h2>
    </div>
  </body>%s
</html>`, response, redirectToMainMenuScript)
	if err != nil {
		log.Print(err)
	}
}

/**
Turn on the fuel cell
*/
func setFcOn(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 2) || (device < 1) {
		log.Print("Invalid fuel cell in 'on' request")
		getStatus(w, r)
		return
	}
	strCommand := fmt.Sprintf("fc on %d", device-1)
	response, err := sendCommand(strCommand)
	if err != nil {
		log.Print(err)
		_, err = fmt.Fprintf(w, err.Error())
		if err != nil {
			log.Print(err)
		}
		return
	}
	_, err = fmt.Fprintf(w, `<html>
  <head>
    <title>Firefly Fuel Cell On</title>
  </head>
  <body>
    <div>%s</div>
	<div>
      <h2>You will be redirected to the status page in a moment.</h2>
    </div>
  </body>%s
</html>`, response, redirectToMainMenuScript)
	if err != nil {
		log.Print(err)
	}
}

/**
Turn off the fuel cell
*/
func setFcOff(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 2) || (device < 1) {
		log.Print("Invalid fuel cell in 'off' request")
		getStatus(w, r)
		return
	}
	device = device - 1
	if err := canBus.fuelCell[uint16(device)].turnOff(); err != nil {
		log.Println("Failed to turn the fuel cell off.", err)
	}
	_, err = fmt.Fprintf(w, `<html>
  <head>
    <title>Firefly Fuel Cell Off</title>
  </head>
  <body>
    <div>Fuel cell stopped and turned off.</div>
	<div>
      <h2>You will be redirected to the status page in a moment.</h2>
    </div>
  </body>%s
</html>`, redirectToMainMenuScript)
	if err != nil {
		log.Print(err)
	}
}

/**
Start the fuel cell
*/
func setFcRun(w http.ResponseWriter, r *http.Request) {
	var jErr JSONError
	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 2) || (device < 1) {
		jErr.AddErrorString("Fuel Cell", "Invalid fuel cell in 'run' request")
		jErr.ReturnError(w, http.StatusBadRequest)
		return
	}
	device = device - 1
	if fc, found := canBus.fuelCell[uint16(device)]; found {
		err = fc.start()
	} else {
		setFcOn(w, r)
		fc, found = canBus.fuelCell[uint16(device)]
		if found {
			err = fc.start()
		} else {
			jErr.AddErrorString("Fuel Cell", "Cell is not transmitting")
			jErr.ReturnError(w, http.StatusInternalServerError)
			return
		}
	}
	if err != nil {
		jErr.AddError("Fuel Cell", err)
		jErr.ReturnError(w, http.StatusInternalServerError)
		return
	}
	_, err = fmt.Fprintf(w, `<html>
  <head>
    <title>Firefly Fuel Cell Start</title>
  </head>
  <body>
    <div>Fuel cell started</div>
	<div>
      <h2>You will be redirected to the status page in a moment.</h2>
    </div>
  </body>%s
</html>`, redirectToMainMenuScript)
	if err != nil {
		log.Print(err)
	}
}

/**
Sop the fuel cell
*/
func setFcStop(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	var jErr JSONError
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 2) || (device < 1) {
		jErr.AddErrorString("Fuel Cell", "Invalid fuel cell in 'on' request")
		jErr.ReturnError(w, 500)
		return
	}
	strCommand := fmt.Sprintf("fc stop %d", device-1)
	response, err := sendCommand(strCommand)
	if err != nil {
		log.Print(err)
		_, err = fmt.Fprintf(w, err.Error())
		if err != nil {
			log.Print(err)
		}
		return
	}
	_, err = fmt.Fprintf(w, `<html>
  <head>
    <title>Firefly Fuel Cell Stop</title>
  </head>
  <body>
    <div>%s</div>
	<div>
      <h2>You will be redirected to the status page in a moment.</h2>
    </div>
  </body>%s
</html>`, response, redirectToMainMenuScript)
	if err != nil {
		log.Print(err)
	}
}

type neuteredFileSystem struct {
	fs http.FileSystem
}

func (nfs neuteredFileSystem) Open(path string) (http.File, error) {
	f, err := nfs.fs.Open(path)
	if err != nil {
		return nil, err
	}

	s, err := f.Stat()
	if s.IsDir() {
		index := filepath.Join(path, "index.html")
		if _, err := nfs.fs.Open(index); err != nil {
			closeErr := f.Close()
			if closeErr != nil {
				return nil, closeErr
			}

			return nil, err
		}
	}

	return f, nil
}

/**
Returns a JSON structure defining the current system contents
*/
func getSystem(w http.ResponseWriter, _ *http.Request) {
	var jErr JSONError
	var System struct {
		Relays          *relayStatus
		NumElectrolyser uint8
		NumFuelCell     uint8
	}
	SystemStatus.m.Lock()
	defer SystemStatus.m.Unlock()
	if !getRelayStatus() {
		jErr.AddErrorString("Relays", "getRelayStatus returned false - not all relays found")
		jErr.ReturnError(w, 500)
		return
	}
	System.Relays = &SystemStatus.Relays
	var err error
	if System.NumElectrolyser, err = settings.GetInt8Setting("Num_EL"); err != nil {
		log.Println(err)
		System.NumElectrolyser = 2
	}
	if System.NumFuelCell, err = settings.GetInt8Setting("Num_FC"); err != nil {
		log.Println(err)
		System.NumFuelCell = 2
	}
	bytesArray, err := json.Marshal(System)
	if err != nil {
		jErr.AddError("Relays", err)
		jErr.ReturnError(w, 500)
		return
	} else {
		_, err = fmt.Fprintf(w, string(bytesArray))
		if err != nil {
			log.Println("getRelays - ", err)
		}
	}
}

/**
getPowerData returns a JSON array containing the energy used and stored based on the given date range
*/
func getPowerData(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	var from time.Time
	var to time.Time
	var jError JSONError
	var sView string
	var err error
	type Row struct {
		Logged string  `json:"logged"`
		Used   float64 `json:"used"`
		Stored float64 `json:"stored"`
	}
	var results []*Row
	now := time.Now()

	// Set the returned type to application/json
	//	w.Header().Set("Content-Type", "application/json")
	//	log.Println("Header set.")

	// Make sure the form date can be parsed
	if from, err = time.ParseInLocation("2006-1-2", vars["from"], now.Location()); err != nil {
		log.Println(err)
		jError.AddError("Power", err)
		jError.ReturnError(w, 400)
		return
	}
	//	log.Println("From time found")
	// Make sure the to date can be parsed
	if to, err = time.ParseInLocation("2006-1-2", vars["to"], now.Location()); err != nil {
		jError.AddError("Power", err)
		jError.ReturnError(w, 400)
		return
	}
	//	log.Println("Got times")
	// Make sure the dates are in the right order
	if to.Before(from) {
		jError.AddErrorString("Power", "from must be before to")
		jError.ReturnError(w, 400)
		return
	}

	// If from date is more than 30 days ago we need to get data from the archive table
	if time.Now().Add(time.Hour * 24 * -30).Before(from) {
		// Get from logging
		if from == to {
			// Only one day requested so get hourly data
			sView = "HourlyPower"
		} else {
			// For multiple days we return daily totals
			sView = "DailyPower"
		}
	} else {
		// Get from logging_archive
		if from == to {
			// For a single day return the hourly date
			sView = "HourlyPowerArchive"
		} else if from.Add(time.Hour * 24 * 30).Before(to) {
			// More than 30 days span so show monthly totals
			sView = "MonthlyPowerArchive"
		} else {
			// Within 30 days so we show daily totals
			sView = "DailyPowerArchive"
		}
	}

	// Build the SQL to send to MariaDB
	var sSql string
	if from.Truncate(time.Hour*24) == time.Now().Truncate(time.Hour*24) {
		// If the date requested is today, then grab the last 24 hours.
		sSql = fmt.Sprintf("SELECT * FROM %s WHERE logged between UNIX_TIMESTAMP('%s') and UNIX_TIMESTAMP('%s')",
			sView, time.Now().Add(time.Hour*-24).Format("2006-01-02 15:00"), time.Now().Format("2006-01-02 15:00"))
	} else {
		sSql = fmt.Sprintf("SELECT * FROM %s WHERE logged between UNIX_TIMESTAMP('%s') and UNIX_TIMESTAMP('%s')",
			sView, from.Format("2006-01-02"), to.Add(time.Hour*24).Format("2006-01-02"))
	}

	// Get the data
	rows, err := pDB.Query(sSql)
	if err != nil {
		if _, err := fmt.Fprintf(w, `{"error":"%s"}`, err.Error()); err != nil {
			log.Println(err)
		}
		return
	}

	// Close the query when we are done
	defer func() {
		if err := rows.Close(); err != nil {
			log.Println("Error closing query - ", err)
		}
	}()

	// For each row we put the values in a JSON struct then add it to the array
	for rows.Next() {
		row := new(Row)
		if err := rows.Scan(&(row.Logged), &(row.Used), &(row.Stored)); err != nil {
			log.Print(err)
		} else {
			results = append(results, row)
		}
	}

	// Marshal the completed array into a byte array to send back to the caller
	if JSON, err := json.Marshal(results); err != nil {
		if _, err := fmt.Fprintf(w, `{"error":"%s"`, err.Error()); err != nil {
			log.Println(err)
		}
	} else {
		// Send the byte array to the caller as text
		if _, err := fmt.Fprintf(w, string(JSON)); err != nil {
			log.Println(err)
		}
	}
}

func calculateCO2Saved(sql string) (float64, string, error) {
	var value float64
	var since string

	rows, err := pDB.Query(sql)
	if err != nil {
		return 0.0, "", err
	}

	// Close the query when we are done
	defer func() {
		if err := rows.Close(); err != nil {
			log.Println("Error closing query - ", err)
		}
	}()

	// For each row we put the values in a JSON struct then add it to the array
	if rows.Next() {
		if err := rows.Scan(&value, &since); err != nil {
			log.Print(err)
			return 0.0, "", err
		} else {
			return math.Round(value*100) / 100, since, nil
		}
	}
	log.Println("No rows returned in calculateCO2Saved")
	return 0.0, "", fmt.Errorf("no rows were returned")
}

func getAvgEnergy() (float64, error) {
	var value float64

	rows, err := pDB.Query(`select round(avg(power)) from (
		select sum(greatest(ifnull(fc1OutputPower, 0) + ifnull(fc2OutputPower, 0), 0)) / 3600 as power
			from logging
			group by date(logged)) as consumption`)
	if err != nil {
		return 0.0, err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Println("Error closing query to get the average energy consumption")
		}
	}()
	// For each row we put the values in a JSON struct then add it to the array
	if rows.Next() {
		if err := rows.Scan(&value); err != nil {
			log.Print(err)
			return 0.0, err
		} else {
			return value, nil
		}
	}
	log.Println("No rows returned in getAvgEnergy")
	return 0.0, fmt.Errorf("no rows were returned")
}

func getCO2Saved(w http.ResponseWriter, _ *http.Request) {
	var Saved struct {
		Active   float64 `json:"active"`
		Archive  float64 `json:"archive"`
		AvgPower float64 `json:"avgPower"`
		Since    string  `json:"since"`
	}
	var jErr JSONError
	var err error

	Saved.Active, Saved.Since, err = calculateCO2Saved(`select ((sum(fc1OutputPower) + ifnull(sum(fc2OutputPower), 0)) / 3600000) * 0.16 as co2, min(logged) as since from logging`)
	if err != nil {
		log.Println(err)
		jErr.AddError("CO2", err)
	}
	Saved.Archive, Saved.Since, err = calculateCO2Saved(`select ((sum(fc1OutputPower) + ifnull(sum(fc2OutputPower), 0)) / 60000) * 0.16 as co2, min(logged) as since from logging_archive`)
	if err != nil {
		log.Println(err)
		jErr.AddError("CO2", err)
	}
	Saved.AvgPower, err = getAvgEnergy()
	if err != nil {
		log.Println(err)
		jErr.AddError("CO2", err)
	}

	bytesArray, err := json.Marshal(Saved)
	if err != nil {
		jErr.AddError("CO2", err)
	}

	if len(jErr.Errors) > 0 {
		jErr.ReturnError(w, 500)
	} else {
		_, err = fmt.Fprintf(w, string(bytesArray))
		if err != nil {
			log.Println(err)
		}
	}
}

func updateSettings(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println(err)
		if _, perr := fmt.Fprintf(w, `<html><head><title>Error</title></head><body>%v</body></html>`, err); perr != nil {
			log.Println(perr)
		}
		return
	}
	holdoffTime := r.Form.Get("elholdoff")
	holdonTime := r.Form.Get("elholdon")
	delayOff := r.Form.Get("eldelayoff")
	delayShutDown := r.Form.Get("elshutdowndelay")

	if len(holdoffTime) > 0 {
		t, err := strconv.Atoi(holdoffTime)
		if err != nil {
			log.Println(err)
		} else {
			params.ElectrolyserHoldOffTime = time.Minute * time.Duration(t)
		}
	}
	if len(holdonTime) > 0 {
		t, err := strconv.Atoi(holdonTime)
		if err != nil {
			log.Println(err)
		} else {
			params.ElectrolyserHoldOnTime = time.Minute * time.Duration(t)
		}
	}
	if len(delayOff) > 0 {
		t, err := strconv.Atoi(delayOff)
		if err != nil {
			log.Println(err)
		} else {
			params.ElectrolyserOffDelay = time.Minute * time.Duration(t)
		}
	}
	if len(delayShutDown) > 0 {
		t, err := strconv.Atoi(delayShutDown)
		if err != nil {
			log.Println(err)
		} else {
			params.ElectrolyserShutDownDelay = time.Minute * time.Duration(t)
		}
	}
	if err := params.WriteSettings(jsonSettings); err != nil {
		log.Println(err)
	}
	getSettings(w, nil)
}

func printOptions(w http.ResponseWriter, setting int64) error {
	var selected string

	for m := int64(1); m < 30; m++ {
		if m == setting {
			selected = "selected"
		} else {
			selected = ""
		}
		_, err := fmt.Fprintf(w, "<option value=%d %s>%d minutes</option>", m, selected, m)
		if err != nil {
			log.Println(err)
			return err
		}
	}
	return nil
}

func getSettings(w http.ResponseWriter, _ *http.Request) {

	_, err := fmt.Fprint(w, `<html>
<head>
	<title>FireflyWeb Settings</title>
	<style>
              .egButton {
                  -moz-box-shadow:inset 5px 5px 0px -2px #a6827e;
                  -webkit-box-shadow:inset 5px 5px 0px -2px #a6827e;
                  box-shadow:inset 5px 5px 0px -2px #a6827e;
                  background:-webkit-gradient(linear, left top, left bottom, color-stop(0.05, #7d5d3b), color-stop(1, #634b30));
                  background:-moz-linear-gradient(top, #7d5d3b 5%, #634b30 100%);
                  background:-webkit-linear-gradient(top, #7d5d3b 5%, #634b30 100%);
                  background:-o-linear-gradient(top, #7d5d3b 5%, #634b30 100%);
                  background:-ms-linear-gradient(top, #7d5d3b 5%, #634b30 100%);
                  background:linear-gradient(to bottom, #7d5d3b 5%, #634b30 100%);
                  filter:progid:DXImageTransform.Microsoft.gradient(startColorstr='#7d5d3b', endColorstr='#634b30',GradientType=0);
                  background-color:#7d5d3b;
                  -webkit-border-radius:12px;
                  -moz-border-radius:12px;
                  border-radius:12px;
                  border:1px solid #54381e;
                  display:inline-block;
                  cursor:pointer;
                  color:#ffffff;
                  font-family:Arial;
                  font-size:16px;
                  padding:12px 26px;
                  text-decoration:none;
              }
              .egButton:hover {
                  background:-webkit-gradient(linear, left top, left bottom, color-stop(0.05, #634b30), color-stop(1, #7d5d3b));
                  background:-moz-linear-gradient(top, #634b30 5%, #7d5d3b 100%);
                  background:-webkit-linear-gradient(top, #634b30 5%, #7d5d3b 100%);
                  background:-o-linear-gradient(top, #634b30 5%, #7d5d3b 100%);
                  background:-ms-linear-gradient(top, #634b30 5%, #7d5d3b 100%);
                  background:linear-gradient(to bottom, #634b30 5%, #7d5d3b 100%);
                  filter:progid:DXImageTransform.Microsoft.gradient(startColorstr='#634b30', endColorstr='#7d5d3b',GradientType=0);
                  background-color:#634b30;
              }
              .egButton:active {
                  position:relative;
                  top:1px;
              }
	</style>
</head>
<body>
	<form action="./settings" method="POST">
		<label for="elholdoff">Electrolyser Hold Off Time</label>
		<select id="elholdoff" name="elholdoff">`)
	err = printOptions(w, int64(params.ElectrolyserHoldOffTime.Minutes()))
	if err != nil {
		log.Println(err)
		return
	}
	_, err = fmt.Fprint(w, `</select><br />
		<label for="elholdon">Electrolyser Hold On Time</label>
		<select id="elholdon" name="elholdon">`)
	if err != nil {
		log.Println(err)
		return
	}
	err = printOptions(w, int64(params.ElectrolyserHoldOnTime.Minutes()))
	if err != nil {
		log.Println(err)
		return
	}
	_, err = fmt.Fprint(w, `</select><br />
		<label for="eldelayoff">Electrolyser Off Delay Time</label>
		<select id="eldelayoff" name="eldelayoff">`)
	if err != nil {
		log.Println(err)
		return
	}
	err = printOptions(w, int64(params.ElectrolyserOffDelay.Minutes()))
	if err != nil {
		log.Println(err)
		return
	}
	_, err = fmt.Fprint(w, `</select><br />
		<label for="elshutdowndelay">Electrolyser Shut Down Delay Time</label>
		<select id="elshutdowndelay" name="elshutdowndelay">`)
	if err != nil {
		log.Println(err)
		return
	}
	err = printOptions(w, int64(params.ElectrolyserShutDownDelay.Minutes()))
	if err != nil {
		log.Println(err)
		return
	}
	_, err = fmt.Fprint(w, `</select><br />
        <button class="egButton" type="submit" >Update Settings</button>
	</form>
</body>
</html>
`)
}

func restartFc(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	var jErr JSONError
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 2) || (device < 1) {
		jErr.AddErrorString("Fuel Cell", "Invalid fuel cell in 'on' request")
		jErr.ReturnError(w, 500)
		return
	}
	fc := canBus.fuelCell[uint16(device-1)]
	go func() {
		if err := fc.turnOff(); err != nil {
			log.Println("Error turning off fuel cell -", err)
		}
	}()
	time.AfterFunc(OFFTIMEFORFUELCELLRESTART, func() {
		if err := fc.start(); err != nil {
			log.Println("Error starting the fuel cell -", err)
		}
	})
	getStatus(w, r)
}

/**
Defines all the available API end points
*/
func setUpWebSite() {
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/ws", startDataWebSocket).Methods("GET")
	router.HandleFunc("/status", getStatus).Methods("GET")
	router.HandleFunc("/fc/{device}/off", setFcOff).Methods("GET", "POST")
	router.HandleFunc("/fc/{device}/on", setFcOn).Methods("GET", "POST")
	router.HandleFunc("/fc/{device}/run", setFcRun).Methods("GET", "POST")
	router.HandleFunc("/fc/{device}/stop", setFcStop).Methods("GET", "POST")
	router.HandleFunc("/fc/{device}/restart", restartFc).Methods("GET", "POST")
	router.HandleFunc("/gas/off", setGasOff).Methods("GET", "POST")
	router.HandleFunc("/gas/on", setGasOn).Methods("GET", "POST")
	router.HandleFunc("/drain/off", setDrainOff).Methods("GET", "POST")
	router.HandleFunc("/drain/on", setDrainOn).Methods("GET", "POST")
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
	router.HandleFunc("/el/setrate", showRateSetter).Methods("GET")
	router.HandleFunc("/el/getRate", getElectrolyserRate).Methods("GET")
	router.HandleFunc("/el/on", setAllElOn).Methods("POST")
	router.HandleFunc("/el/off", setAllElOff).Methods("POST")
	router.HandleFunc("/dr/{device}/status", getDryerJsonStatus).Methods("GET")
	router.HandleFunc("/fc/{device}/status", getFuelCellJsonStatus).Methods("GET")
	router.HandleFunc("/minStatus", getMinHtmlStatus).Methods("GET")
	router.HandleFunc("/fcdata/{from}/{to}", getFuelCellHistory).Methods("GET")
	router.HandleFunc("/eldata/{from}/{to}", getElectrolyserHistory).Methods("GET")
	router.HandleFunc("/powerdata/{from}/{to}", getPowerData).Methods("GET")
	router.HandleFunc("/co2saved", getCO2Saved).Methods("GET")
	router.HandleFunc("/fcerrors", getFuelCellErrors).Methods("GET")
	router.HandleFunc("/fcdetail/{device}/{from}", getFuelCellDetail).Methods("GET")
	router.HandleFunc("/eldetail/{device}/{from}/{to}", getElectrolyserDetail).Methods("GET")
	router.HandleFunc("/system", getSystem).Methods("GET")
	router.HandleFunc("/candump/{from}/{to}", candump).Methods("GET")
	router.HandleFunc("/candumpEvent/{event}", candumpEvent).Methods("GET")
	router.HandleFunc("/canrecord/{to}", canRecord).Methods("GET")
	router.HandleFunc("/canEvents", listCANEvents).Methods("GET")
	router.HandleFunc("/settings", getSettings).Methods("GET")
	router.HandleFunc("/settings", updateSettings).Methods("POST")
	fileServer := http.FileServer(neuteredFileSystem{http.Dir("./web")})
	router.PathPrefix("/").Handler(http.StripPrefix("/", fileServer))

	log.Fatal(http.ListenAndServe(":20080", router))
}

/**
Function to read the responses form the esm command line application
*/
func commandResponseReader(outPipe *bufio.Reader) {
	for {
		text, err := outPipe.ReadString('>')
		if err != nil {
			log.Println("CommandResponseReader error - ", err)
			return
		}
		if strings.Trim(text, " ") != string(esmPrompt) {
			commandResponse <- text
		}
	}
}

func init() {
	var settingFile string

	SystemStatus.valid = false // Prevents logging until we have some actual data
	// Set up logging
	logwriter, e := syslog.New(syslog.LOG_NOTICE, "FireflyWeb")
	if e == nil {
		log.SetOutput(logwriter)
	}

	// Get the settings
	flag.StringVar(&databaseServer, "sqlServer", "localhost", "MySQL Server")
	flag.StringVar(&databaseName, "database", "firefly", "Database name")
	flag.StringVar(&databaseLogin, "dbUser", "FireflyService", "Database login user name")
	flag.StringVar(&databasePassword, "dbPassword", "logger", "Database user password")
	flag.StringVar(&databasePort, "dbPort", "3306", "Database port")
	flag.StringVar(&executable, "exec", "./esm-3.17.13", "Path to the FireFly esm executable")
	flag.BoolVar(&debug, "debug", false, "Set debug=true to output move information ot the log file")
	flag.StringVar(&CANInterface, "can", "can0", "CAN Interface Name")
	flag.StringVar(&settingFile, "settings", "/esm/system.config", "Path to the settings file")
	flag.StringVar(&jsonSettings, "jsonSettings", "/etc/FireFlyWeb.json", "JSON file containing the system control parameters")
	flag.Parse()
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	if debug {
		log.Println("running in debug mode")
	} else {
		log.Println("running in non-debug mode")
	}

	if err := settings.ReadSettings(settingFile); err != nil {
		log.Println("Error reading settings file - ", err)
	}

	params = NewJsonSettings()
	if err := params.ReadSettings(jsonSettings); err != nil {
		log.Panic("Error reading the JSON settings file - ", err)
	}

	commandResponse = make(chan string)
	esmCommand.valid = false
	var err error

	if pDB, err = connectToDatabase(); err != nil {
		log.Println(`Cannot connect to the database - `, err)
	}

	canBus = initCANLogger(systemConfig.NumFc)

	esmCommand.command = exec.Command(executable)
	esmCommand.stdin, err = esmCommand.command.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}

	esmCommand.stdout, err = esmCommand.command.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	outPipe := bufio.NewReader(esmCommand.stdout)
	if err = esmCommand.command.Start(); err != nil {
		log.Fatal(err)
	}

	go commandResponseReader(outPipe)

	esmCommand.valid = true
	time.Sleep(2 * time.Second)

	go setUpWebSite()

	// Calculate the time we should start trying to turn the electorlysers off and archive the old data
	CalculateOffTime()

}

func loggingLoop() {
	done := make(chan bool)
	loggingTime := time.NewTicker(time.Second)
	fcPolling := time.NewTicker(time.Millisecond * 200)

	for {
		select {
		case <-done:
			return
		case <-loggingTime.C:
			{
				getSystemStatus()
				if SystemStatus.valid {
					logStatus()
					for _, fc := range canBus.fuelCell {
						fc.checkFuelCell() // Check for errors and reset the fuel cell if there are any.
					}
				}
				signal.Broadcast()
				if time.Now().After(electrolyserShutDownTime) {
					ShutDownElectrolysers()
				}
				h, _, s := time.Now().Clock()
				if h == 1 && s == 0 && electrolyserShutDownTime.Before(time.Now()) {
					// At 1AM we recaulculate the shut off time and archive the old data
					// Repeat until the shutdown time has been updated to later on today
					go CalculateOffTime()
				}
			}
		case <-fcPolling.C:
			{
				logFuelCellData()
			}
		}
	}
}

func main() {
	startup := <-commandResponse
	_, err := fmt.Println(startup)
	if err != nil {
		log.Print(err)
	}

	getSystemInfo()
	log.Println("FireflyWeb Starting with ", systemConfig.NumFc, " Fuelcells : ", systemConfig.NumEl, " Electrolysers : ", systemConfig.NumDryer, " Dryers")
	signal = sync.NewCond(&sync.Mutex{})

	loggingLoop()
}
