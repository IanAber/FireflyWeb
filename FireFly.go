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
	"runtime"

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
const ELECTROLYSERMAXSTACKVOLTSFORTURNOFF = 30 // Cannot turn off the electrolyser above this voltage
const MAXFUELCELLRESTARTS = 10
const OFFTIMEFORFUELCELLRESTART = time.Second * 20
const FUELCELLENABLETORUNDELAY = time.Second * 2
const GASONDELAY = time.Second * 2
const convertPSIToBar = 14.503773773
const CANDUMPSQL = `SELECT UNIX_TIMESTAMP(logged) AS logged, ID, canData FROM CAN_Data WHERE logged BETWEEN ? AND ?`
const CANDUMPEVENTSQL = `SELECT UNIX_TIMESTAMP(logged) AS logged, ID, canData FROM CAN_Data WHERE Event = ?`
const LISTCANEVENTSSQL = `SELECT DISTINCT Event, OnDemand FROM CAN_Data WHERE Event IS NOT NULL ORDER BY Event DESC LIMIT 50`
const redirectToMainMenuScript = `
<script>
	var tID = setTimeout(function () {
		window.location.href = "/status";
		window.clearTimeout(tID);		// clear time out.
	}, 2000);
</script>
`
const SUCCESSJSONRESPONCE = `{"success":true}`

type OnOffBody struct {
	State bool `json:"state"`
}

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

func debugPrint(format string, args ...interface{}) {
	if params.DebugOutput {
		var sErr string
		if len(args) > 0 {
			sErr = fmt.Sprintf(format, args...)
		} else {
			sErr = format
		}
		_, function, line, ok := runtime.Caller(1)
		if ok {
			parts := strings.Split(function, "/")
			log.Printf("%s:%d | %s", parts[len(parts)-1], line, sErr)
		} else {
			log.Print(sErr)
		}
	}
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
	if (err != nil) || (device > 1) || (device < 0) {
		log.Print("Invalid electrolyser in set production rate request")
		ReturnJSONErrorString(w, "electrolyser", "Invalid electrolyser in set production rate request", http.StatusBadRequest, true)
		return
	}
	currentRate := int8(SystemStatus.Electrolysers[device].GetRate())
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

func returnJSONSuccess(w http.ResponseWriter) {
	if _, err := fmt.Fprintf(w, SUCCESSJSONRESPONCE); err != nil {
		log.Println(err)
	}
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
	} else if !strings.Contains(val, "OFF") && !strings.Contains(val, "UNKNOWN") {
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

func getRelayStatus() bool {
	text, err := sendCommand("relay status")
	if err != nil {
		log.Println(err)
		return false
	}
	return populateRelayData(text)
}

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

	if !getRelayStatus() {
		ReturnJSONErrorString(w, "status", `<head><title>Firefly Status Error</title></head>
<body><h1>ERROR fetching relay status.</h1><br />
<h3>One or more relays could not be identified in the "relay status" command.</h3>
</body></html>`, http.StatusInternalServerError, true)
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
		log.Print(err)
	}
	for idx, el := range SystemStatus.Electrolysers {
		if _, err := fmt.Fprintf(w, `<div><h2>Electrolyser %d</h2>%s</div>`, idx, getElectrolyserHtmlStatus(el)); err != nil {
			log.Print(err)
		}
	}
	if _, err := fmt.Fprintf(w, `<div><div style="float:left; width:48%%"><h2>Dryer</h2>%s</div>`, getDryerHtmlStatus(SystemStatus.Electrolysers[0])); err != nil {
		log.Print(err)
	}
	if params.FuelCellMaintenance {
		if _, err := fmt.Fprintf(w, `<div style="float:left; width:48%%"><h2>Fuel Cell Maintenance Mode Enabled</h2></div>`); err != nil {
			log.Print(err)
		}
	} else {
		for idx, fc := range canBus.fuelCell {
			if _, err := fmt.Fprintf(w, `<div style="float:left; width:48%%"><h2>Fuel Cell %d</h2>%s<br /><a href="/fc/%d/restart">Restart</a></div>`, idx, getFuelCellHtmlStatus(fc), idx); err != nil {
				log.Print(err)
			}
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
		log.Print(err)
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
Get the dryer status as a JSON object
*/
func getDryerJsonStatus(w http.ResponseWriter, r *http.Request) {
	// Set the returned type to application/json
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 1) || (device < 0) {
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
			f := enableElectrolyser(device)
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
			params.fc1AnodePressure.Float64 = float64(fc.getAnodePressure())
			params.fc1AnodePressure.Valid = true
			params.fc1FaultFlagA = fc.getFaultA()
			params.fc1FaultFlagB = fc.getFaultB()
			params.fc1FaultFlagC = fc.getFaultC()
			params.fc1FaultFlagD = fc.getFaultD()
			params.fc1InletTemp.Float64 = float64(fc.getInletTemp())
			params.fc1InletTemp.Valid = true
			params.fc1OutletTemp.Float64 = float64(fc.getOutletTemp())
			params.fc1OutletTemp.Valid = true
			params.fc1OutputCurrent.Float64 = float64(fc.getOutputCurrent())
			params.fc1OutputCurrent.Valid = true
			params.fc1OutputVoltage.Float64 = float64(fc.getOutputVolts())
			params.fc1OutputVoltage.Valid = true
			params.fc1OutputPower = fc.getOutputPower()
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
			params.fc2FaultFlagA = fc.getFaultA()
			params.fc2FaultFlagB = fc.getFaultB()
			params.fc2FaultFlagC = fc.getFaultC()
			params.fc2FaultFlagD = fc.getFaultD()
			params.fc2InletTemp.Float64 = float64(fc.getInletTemp())
			params.fc2InletTemp.Valid = true
			params.fc2OutletTemp.Float64 = float64(fc.getOutletTemp())
			params.fc2OutletTemp.Valid = true
			params.fc2OutputCurrent.Float64 = float64(fc.getOutputCurrent())
			params.fc2OutputCurrent.Valid = true
			params.fc2OutputVoltage.Float64 = float64(fc.getOutputVolts())
			params.fc2OutputVoltage.Valid = true
			params.fc1OutputPower = fc.getOutputPower()
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
		minFc.Output = float32(fc.getOutputPower())
		minFc.Alarm = getAllFuelCellErrors(fc.getFaultA(), fc.getFaultB(), fc.getFaultC(), fc.getFaultD())
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

/**
Get running status as a JSON object
*/
func getFullJsonStatus() string {
	SystemStatus.m.Lock()
	defer SystemStatus.m.Unlock()

	type ElectrolyserStatus struct {
		On                    bool        `json:"on"`
		State                 string      `json:"state"`
		Serial                string      `json:"serial"`
		SystemState           string      `json:"systemstate"`
		H2Flow                jsonFloat32 `json:"h2flow"`
		ElState               string      `json:"elstate"`
		ElectrolyteLevel      string      `json:"level"`
		StackCurrent          jsonFloat32 `json:"current"`
		StackVoltage          jsonFloat32 `json:"voltage"`
		InnerH2Pressure       jsonFloat32 `json:"innerpressure"`
		OuterH2Pressure       jsonFloat32 `json:"outerpressure"`
		WaterPressure         jsonFloat32 `json:"waterpressure"`
		ElectrolyteTemp       jsonFloat32 `json:"temp"`
		CurrentProductionRate int         `json:"rate"`
		DefaultProductionRate int         `json:"defrate"`
		MaxTankPressure       jsonFloat32 `json:"maxtank"`
		RestartPressure       jsonFloat32 `json:"restart"`
		Warnings              string      `json:"warnings"`
		Errors                string      `json:"errors"`
	}
	type DryerStatus struct {
		On             bool        `json:"on"`
		Temp0          jsonFloat32 `json:"temp0"`
		Temp1          jsonFloat32 `json:"temp1"`
		Temp2          jsonFloat32 `json:"temp2"`
		Temp3          jsonFloat32 `json:"temp3"`
		InputPressure  jsonFloat32 `json:"inputPressure"`
		OutputPressure jsonFloat32 `json:"outputPressure"`
		Errors         string      `json:"errors"`
		Warnings       string      `json:"warnings"`
	}
	type FuelCellStatus struct {
		On            bool        `json:"on"`
		State         string      `json:"state"`
		Power         int16       `json:"power"`
		Volts         jsonFloat32 `json:"volts"`
		Amps          jsonFloat32 `json:"amps"`
		FaultA        string      `json:"faultA"`
		FaultB        string      `json:"faultB"`
		FaultC        string      `json:"faultC"`
		FaultD        string      `json:"faultD"`
		AnodePressure jsonFloat32 `json:"anodePressure"`
		InletTemp     jsonFloat32 `json:"inletTemp"`
		OutletTemp    jsonFloat32 `json:"outletTemp"`
		Serial        string      `json:"serial"`
		Version       string      `json:"version"`
	}

	type GasStatus struct {
		FuelCellPressure jsonFloat32 `json:"fcpressure"`
		TankPressure     jsonFloat32 `json:"tankpressure"`
	}

	type RelaysStatus struct {
		El0       bool `json:"el0"`
		El1       bool `json:"el1"`
		Gas       bool `json:"gas"`
		FC0Enable bool `json:"fc0en"`
		FC0Run    bool `json:"fc0run"`
		FC1Enable bool `json:"fc1en"`
		FC1Run    bool `json:"fc1run"`
		Drain     bool `json:"drain"`
	}

	var Status struct {
		Relays        RelaysStatus          `json:"relays"`
		Electrolysers []*ElectrolyserStatus `json:"el"`
		Dryer         DryerStatus           `json:"dr"`
		FuelCells     []*FuelCellStatus     `json:"fc"`
		Gas           GasStatus             `json:"gas"`
		Tds           int64                 `json:"tds"`
	}
	Status.Gas.FuelCellPressure = jsonFloat32(math.Round(float64(SystemStatus.Gas.FuelCellPressure * 1000)))
	Status.Gas.TankPressure = jsonFloat32(math.Round(SystemStatus.Gas.TankPressure*10) / 10)
	Status.Relays.Gas = SystemStatus.Relays.GasToFuelCell
	Status.Relays.El0 = SystemStatus.Relays.Electrolyser1
	Status.Relays.El1 = SystemStatus.Relays.Electrolyser2
	Status.Relays.FC0Enable = SystemStatus.Relays.FuelCell1Enable
	Status.Relays.FC0Run = SystemStatus.Relays.FuelCell1Run
	Status.Relays.FC1Enable = SystemStatus.Relays.FuelCell2Enable
	Status.Relays.FC1Run = SystemStatus.Relays.FuelCell2Run
	Status.Relays.Drain = SystemStatus.Relays.Drain
	Status.Tds = SystemStatus.TDS.TdsReading
	Status.Dryer.On = false
	Status.Dryer.Errors = ""
	Status.Dryer.Temp0 = 0
	Status.Dryer.Temp1 = 0
	Status.Dryer.Temp2 = 0
	Status.Dryer.Temp3 = 0
	Status.Dryer.InputPressure = 0
	Status.Dryer.OutputPressure = 0
	Status.Dryer.Warnings = ""
	for elnum, el := range SystemStatus.Electrolysers {
		ElStatus := new(ElectrolyserStatus)
		if elnum == 0 {
			ElStatus.On = SystemStatus.Relays.Electrolyser1
		} else {
			ElStatus.On = SystemStatus.Relays.Electrolyser2
		}
		if ElStatus.On {
			ElStatus.Serial = el.status.Serial
			ElStatus.ElState = el.getState()
			ElStatus.H2Flow = jsonFloat32(math.Round(float64(el.status.H2Flow*10)) / 10)
			ElStatus.SystemState = el.GetSystemState()
			ElStatus.ElectrolyteLevel = el.status.ElectrolyteLevel.String()
			ElStatus.StackCurrent = jsonFloat32(math.Round(float64(el.status.StackCurrent*10)) / 10)
			ElStatus.StackVoltage = jsonFloat32(math.Round(float64(el.status.StackVoltage*10)) / 10)
			ElStatus.InnerH2Pressure = jsonFloat32(math.Round(float64(el.status.InnerH2Pressure*10)) / 10)
			ElStatus.OuterH2Pressure = jsonFloat32(math.Round(float64(el.status.OuterH2Pressure*10)) / 10)
			ElStatus.WaterPressure = jsonFloat32(math.Round(float64(el.status.WaterPressure*10)) / 10)
			ElStatus.ElectrolyteTemp = jsonFloat32(math.Round(float64(el.status.ElectrolyteTemp*10)) / 10)
			ElStatus.CurrentProductionRate = int(el.status.CurrentProductionRate)
			ElStatus.DefaultProductionRate = int(el.status.DefaultProductionRate)
			ElStatus.MaxTankPressure = jsonFloat32(math.Round(float64(el.status.MaxTankPressure*10)) / 10)
			ElStatus.RestartPressure = jsonFloat32(math.Round(float64(el.status.RestartPressure*10)) / 10)
			ElStatus.Warnings = strings.Join(el.GetWarnings(), ":")
			ElStatus.Errors = strings.Join(el.GetErrors(), ":")
		}
		Status.Electrolysers = append(Status.Electrolysers, ElStatus)

		// If this is the first electrolyser get the dryer details from it
		if elnum == 0 {
			Status.Dryer.On = el.status.SwitchedOn
			Status.Dryer.InputPressure = jsonFloat32(math.Round(float64(el.status.DryerInputPressure*10)) / 10)
			Status.Dryer.OutputPressure = jsonFloat32(math.Round(float64(el.status.DryerOutputPressure*10)) / 10)
			Status.Dryer.Temp0 = jsonFloat32(math.Round(float64(el.status.DryerTemp1*10)) / 10)
			Status.Dryer.Temp1 = jsonFloat32(math.Round(float64(el.status.DryerTemp2*10)) / 10)
			Status.Dryer.Temp2 = jsonFloat32(math.Round(float64(el.status.DryerTemp3*10)) / 10)
			Status.Dryer.Temp3 = jsonFloat32(math.Round(float64(el.status.DryerTemp4*10)) / 10)
			Status.Dryer.Errors = el.GetDryerErrorText()
			Status.Dryer.Warnings = el.GetDryerWarningText()
		}
	}
	for _, fc := range canBus.fuelCell {
		FcStatus := new(FuelCellStatus)
		FcStatus.On = fc.IsSwitchedOn()
		FcStatus.Version = fmt.Sprintf("%d.%d.%d", fc.Software.Version, fc.Software.Major, fc.Software.Minor)
		FcStatus.Serial = string(fc.Serial[:])
		FcStatus.InletTemp = jsonFloat32(math.Round(float64(fc.InletTemp*10)) / 10)
		FcStatus.OutletTemp = jsonFloat32(math.Round(float64(fc.OutletTemp*10)) / 10)
		FcStatus.Power = fc.OutputPower
		FcStatus.Amps = jsonFloat32(math.Round(float64(fc.OutputCurrent*10)) / 10)
		FcStatus.Volts = jsonFloat32(math.Round(float64(fc.OutputVolts*10)) / 10)
		FcStatus.State = fc.GetState()
		FcStatus.FaultA = strings.Join(getFuelCellError('A', fc.getFaultA()), ":")
		FcStatus.FaultB = strings.Join(getFuelCellError('B', fc.getFaultB()), ":")
		FcStatus.FaultC = strings.Join(getFuelCellError('C', fc.getFaultC()), ":")
		FcStatus.FaultD = strings.Join(getFuelCellError('D', fc.getFaultD()), ":")
		FcStatus.AnodePressure = jsonFloat32(math.Round(float64(fc.AnodePressure*10)) / 10)

		Status.FuelCells = append(Status.FuelCells, FcStatus)
	}

	bytes, err := json.Marshal(Status)
	if err != nil {
		log.Print(err)
	}
	return string(bytes)
}

func getMinHtmlStatus(w http.ResponseWriter, _ *http.Request) {
	// Set the returned type to application/json
	w.Header().Set("Content-Type", "application/json")

	if _, err := fmt.Fprint(w, getMinJsonStatus()); err != nil {
		log.Println("Error getting MinHtmlStatus - ", err)
	}
}

/**
Turn the fuel cell gas on
*/
func setGas(w http.ResponseWriter, r *http.Request) {
	var body OnOffBody

	if bytes, err := io.ReadAll(r.Body); err != nil {
		ReturnJSONError(w, "Gas", err, http.StatusInternalServerError, true)
		return
	} else {
		debugPrint(string(bytes))
		if err := json.Unmarshal(bytes, &body); err != nil {
			ReturnJSONError(w, "Gas", err, http.StatusBadRequest, true)
			return
		}
	}
	var err error
	if body.State {
		err = turnOnGas()
	} else {
		err = turnOffGas()
	}
	if err != nil {
		ReturnJSONError(w, "Gas", err, http.StatusInternalServerError, true)
		return
	}
	returnJSONSuccess(w)
}

/**
Turn on the drain solenoid
*/
func setDrain(w http.ResponseWriter, r *http.Request) {

	var body OnOffBody
	var strCommand string

	if bytes, err := io.ReadAll(r.Body); err != nil {
		ReturnJSONError(w, "Drain", err, http.StatusInternalServerError, true)
		return
	} else {
		debugPrint(string(bytes))
		if err := json.Unmarshal(bytes, &body); err != nil {
			ReturnJSONError(w, "Drain", err, http.StatusBadRequest, true)
			return
		}
	}
	if body.State {
		strCommand = "relay drain on"
	} else {
		strCommand = "relay drain off"
	}
	if _, err := sendCommand(strCommand); err != nil {
		ReturnJSONError(w, "Drain", err, http.StatusInternalServerError, true)
		return
	}
	returnJSONSuccess(w)
}

/**
Turn off the drain solenoid
*/
//func setDrainOff(w http.ResponseWriter, _ *http.Request) {
//	_, err := sendCommand("drain off")
//	if err != nil {
//		log.Print(err)
//		_, err = fmt.Fprintf(w, err.Error())
//		if err != nil {
//			log.Print(err)
//		}
//		return
//	}
//	returnJSONSuccess(w)
//}

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
	var System struct {
		Relays              *relayStatus
		NumElectrolyser     uint8
		NumFuelCell         uint8
		FuelCellMaintenance bool
	}
	SystemStatus.m.Lock()
	defer SystemStatus.m.Unlock()
	if !getRelayStatus() {
		ReturnJSONErrorString(w, "Relays", "getRelayStatus returned false - not all relays found", http.StatusInternalServerError, true)
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
	System.FuelCellMaintenance = params.FuelCellMaintenance
	bytesArray, err := json.Marshal(System)
	if err != nil {
		ReturnJSONError(w, "Relays", err, http.StatusInternalServerError, true)
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
		ReturnJSONError(w, "Power", err, http.StatusBadRequest, true)
		return
	}
	//	log.Println("From time found")
	// Make sure the to date can be parsed
	if to, err = time.ParseInLocation("2006-1-2", vars["to"], now.Location()); err != nil {
		ReturnJSONError(w, "Power", err, http.StatusBadRequest, true)
		return
	}
	//	log.Println("Got times")
	// Make sure the dates are in the right order
	if to.Before(from) {
		ReturnJSONErrorString(w, "Power", "from must be before to", http.StatusBadRequest, true)
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
	var err error

	Saved.Active, Saved.Since, err = calculateCO2Saved(`select ((sum(fc1OutputPower) + ifnull(sum(fc2OutputPower), 0)) / 3600000) * 0.16 as co2, min(logged) as since from logging`)
	if err != nil {
		ReturnJSONError(w, "CO2", err, http.StatusInternalServerError, true)
		return
	}
	Saved.Archive, Saved.Since, err = calculateCO2Saved(`select ((sum(fc1OutputPower) + ifnull(sum(fc2OutputPower), 0)) / 60000) * 0.16 as co2, min(logged) as since from logging_archive`)
	if err != nil {
		ReturnJSONError(w, "CO2", err, http.StatusInternalServerError, true)
		return
	}
	Saved.AvgPower, err = getAvgEnergy()
	if err != nil {
		ReturnJSONError(w, "CO2", err, http.StatusInternalServerError, true)
		return
	}

	bytesArray, err := json.Marshal(Saved)
	if err != nil {
		ReturnJSONError(w, "CO2", err, http.StatusInternalServerError, true)
		return
	}

	_, err = fmt.Fprintf(w, string(bytesArray))
	if err != nil {
		log.Println(err)
	}
}

/***
turnOnGas first checks then turns on the gas if it is off. It delays 2 seconds if the gas was off
*/
func turnOnGas() error {
	if SystemStatus.Relays.GasToFuelCell {
		// Gas is already on
		return nil
	}
	if _, err := sendCommand("gas on"); err != nil {
		log.Print(err)
		return err
	}
	time.Sleep(time.Second * 2)
	return nil
}

func turnOffGas() error {
	if !SystemStatus.Relays.GasToFuelCell {
		// Gas is already off
		return nil
	}
	if _, err := sendCommand("gas off"); err != nil {
		log.Print(err)
		return err
	}
	time.Sleep(time.Second)
	return nil
}

/**
Defines all the available API end points
*/
func setUpWebSite() {
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/ws", startDataWebSocket).Methods("GET")
	router.HandleFunc("/wsFull", startStatusWebSocket).Methods("GET")
	router.HandleFunc("/status", getStatus).Methods("GET")
	router.HandleFunc("/fcerrors", getFuelCellErrors).Methods("GET")
	router.HandleFunc("/fcdetail/{device}/{from}", getFuelCellDetail).Methods("GET")
	router.HandleFunc("/fcdata/{from}/{to}", getFuelCellHistory).Methods("GET")
	router.HandleFunc("/fc/{device}/off", setFcOff).Methods("GET", "POST")
	router.HandleFunc("/fc/{device}/on", setFcOn).Methods("GET", "POST")
	router.HandleFunc("/fc/{device}/run", setFcRun).Methods("GET", "POST")
	router.HandleFunc("/fc/{device}/stop", setFcStop).Methods("GET", "POST")
	router.HandleFunc("/fc/{device}/restart", restartFc).Methods("GET", "POST")
	router.HandleFunc("/fc/{device}/status", fcStatus).Methods("GET")
	router.HandleFunc("/fc/{device}/on_off", setFcOnOff).Methods("POST")
	router.HandleFunc("/fc/maintenance", setFcMaintenance).Methods("POST")
	router.HandleFunc("/gas", setGas).Methods("PUT")
	router.HandleFunc("/drain", setDrain).Methods("PUT")
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
	router.HandleFunc("/el/setrate", showRateSetter).Methods("GET")
	router.HandleFunc("/el/getRate", getElectrolyserRate).Methods("GET")
	router.HandleFunc("/el/on", setAllElOn).Methods("POST")
	router.HandleFunc("/el/off", setAllElOff).Methods("POST")
	router.HandleFunc("/dr/{device}/status", getDryerJsonStatus).Methods("GET")
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
	flag.StringVar(&CANInterface, "can", "can0", "CAN Interface Name")
	flag.StringVar(&settingFile, "settings", "/esm/system.config", "Path to the settings file")
	flag.StringVar(&jsonSettings, "jsonSettings", "/etc/FireFlyWeb.json", "JSON file containing the system control parameters")
	flag.Parse()
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	if err := settings.ReadSettings(settingFile); err != nil {
		log.Println("Error reading settings file - ", err)
	}

	params = NewJsonSettings()
	if err := params.ReadSettings(jsonSettings); err != nil {
		log.Panic("Error reading the JSON settings file - ", err)
	}
	if params.DebugOutput {
		log.Println("running in debug mode")
	} else {
		log.Println("running in non-debug mode")
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
				dataSignal.Broadcast()
				statusSignal.Broadcast()
				if time.Now().After(electrolyserShutDownTime) {
					ShutDownElectrolysers()
				}
				h, _, s := time.Now().Clock()
				if h == 1 && s == 0 && electrolyserShutDownTime.Before(time.Now()) {
					// At 1AM we recalculate the shutoff time and archive the old data
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
	dataSignal = sync.NewCond(&sync.Mutex{})
	statusSignal = sync.NewCond(&sync.Mutex{})

	loggingLoop()
}
