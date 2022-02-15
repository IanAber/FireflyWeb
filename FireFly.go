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
	"github.com/gorilla/websocket"
	"io"
	"log"
	"log/syslog"
	"math"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const convertPSIToBar = 14.503773773
const ElIdle = "EL_STATE_IDLE"
const redirectToMainMenuScript = `
<script>
	var tID = setTimeout(function () {
		window.location.href = "/status";
		window.clearTimeout(tID);		// clear time out.
	}, 2000);
</script>
`

type electrolyserStatus struct {
	Device                int64
	IP                    string
	Model                 string
	Firmware              string
	Serial                string
	SystemState           string
	H2Flow                float64
	ElState               string
	ElectrolyteLevel      string
	InnerH2Pressure       float64
	OuterH2Pressure       float64
	WaterPressure         float64
	ElectrolyteTemp       float64
	CurrentProductionRate int64
	DefaultProductionRate int64
	MaxTankPressure       int64
	RestartPressure       int64
	StackVoltage          float64
	StackCurrent          float64
}

type dryerStatus struct {
	Status         string
	DryerTemp0     float64
	DryerTemp1     float64
	DryerTemp2     float64
	DryerTemp3     float64
	InputPressure  float64
	OutputPressure float64
	DryerError     string
	DryerWarning   string
}

type gasStatus struct {
	FuelCellPressure float64
	TankPressure     float64
}

type fuelCellStatus struct {
	SerialNumber  string
	Version       string
	OutputPower   float64
	OutputVolt    float64
	OutputCurrent float64
	AnodePress    float64
	InletTemp     float64
	OutletTemp    float64
	State         string
	FaultFlagA    string
	FaultFlagB    string
	FaultFlagC    string
	FaultFlagD    string
}

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
		m             sync.Mutex
		valid         bool
		Relays        relayStatus
		FuelCells     []*fuelCellStatus
		Electrolysers []*electrolyserStatus
		Dryers        []*dryerStatus
		Gas           gasStatus
		TDS           tdsStatus
	}
)

var systemConfig struct {
	consoleHistory             int64
	NumDryer                   int16
	NumEl                      int16
	ElAddresses                string
	NumFc                      int16
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

func connectToDatabase() (*sql.DB, error) {
	if pDB != nil {
		_ = pDB.Close()
		pDB = nil
	}
	var sConnectionString = databaseLogin + ":" + databasePassword + "@tcp(" + databaseServer + ":" + databasePort + ")/" + databaseName

	//	fmt.Println("Connecting to [", sConnectionString, "]")
	db, err := sql.Open("mysql", sConnectionString)
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, err
}

func showElectrolyserProductionRatePage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 2) || (device < 1) {
		log.Print("Invalid electrolyser in reboot request")
		var jErr JSONError
		jErr.AddErrorString("electrolyser", "Invalid electrolyser in reboot request")
		jErr.ReturnError(w, 400)
		return
	}
	currentRate := SystemStatus.Electrolysers[device-1].CurrentProductionRate
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
	for v := int64(100); v > 59; v-- {
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

func populateFuelCellData(text string) (status *fuelCellStatus) {
	valueLines := getKeyValueLines(text, ": ")
	if len(valueLines) > 0 {
		status = new(fuelCellStatus)
		for _, valueLine := range valueLines {
			keyValue := strings.Split(valueLine, ": ")
			key := strings.Trim(keyValue[0], " ")
			value := strings.Trim(keyValue[1], " ")
			//			fmt.Println("Key =", key, "\nValue = ", value, "\nNumeric = ", strings.TrimFunc(value, notNumeric))
			switch key {
			case "Serial Number":
				status.SerialNumber = strings.Trim(value, "\u0000")
			case "Version":
				status.Version = value
			case "Output Power":
				status.OutputPower, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 32)
			case "Output Volt":
				status.OutputVolt, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 32)
			case "Output Current":
				status.OutputCurrent, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 32)
			case "Anode Press":
				status.AnodePress, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 32)
			case "Inlet Temp":
				status.InletTemp, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 32)
			case "Outlet Temp":
				status.OutletTemp, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 32)
			case "State":
				status.State = value
				//				fmt.Println("Value = ", value, "\nState = ", status.State)
			case "Fault Flag_A":
				status.FaultFlagA = value
			case "Fault Flag_B":
				status.FaultFlagB = value
			case "Fault Flag_C":
				status.FaultFlagC = value
			case "Fault Flag_D":
				status.FaultFlagD = value
			default:
				fmt.Printf("Fuelcell info returned >>>>> [%s]\n", valueLine)
			}
		}
	}
	return
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
			//			fmt.Println("Key =", key, "\nValue = ", value, "\nNumeric = ", strings.TrimFunc(value, notNumeric))
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
	} else {
		return false
	}
}

func populateSystemInfo(text string) {
	valueLines := getKeyValueLines(text, " = ")
	systemConfig.NumDryer = -1
	systemConfig.NumFc = -1
	systemConfig.NumEl = -1
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
				systemConfig.NumDryer = int16(n)
			case "Num_EL":
				n, _ := strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 16)
				systemConfig.NumEl = int16(n)
			case "Num_FC":
				n, _ := strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 16)
				systemConfig.NumFc = int16(n)
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

				//				LogOverwrite
				//				SimMode

			}
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
			//			fmt.Println("Key =", key, "\nValue = ", value, "\nNumeric = ", strings.TrimFunc(value, notNumeric))
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
			//			fmt.Println("Key =", key, "\nValue = ", value, "\nNumeric = ", strings.TrimFunc(value, notNumeric))
			switch key {
			case "TDS reading":
				SystemStatus.TDS.TdsReading, _ = strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 64)
			}
		}
	}
	return
}

func populateDryerData(text string) (status *dryerStatus) {
	valueLines := getKeyValueLines(text, ": ")
	status = new(dryerStatus)
	status.Status = "Active"
	if len(valueLines) > 0 {
		for _, valueLine := range valueLines {
			keyValue := strings.Split(valueLine, ":")
			if len(keyValue) < 2 {
				return
			}
			key := strings.Trim(keyValue[0], " ")
			value := strings.Trim(keyValue[1], " ")
			//			fmt.Println("Key =", key, "\nValue = ", value, "\nNumeric = ", strings.TrimFunc(value, notNumeric))
			switch key {
			case "Dryer Temp 0":
				status.DryerTemp0, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 32)
			case "Dryer Temp 1":
				status.DryerTemp1, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 32)
			case "Dryer Temp 2":
				status.DryerTemp2, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 32)
			case "Dryer Temp 3":
				status.DryerTemp3, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 32)
			case "Input Pressure":
				status.InputPressure, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 32)
			case "Output Pressure":
				status.OutputPressure, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 32)
			case "Dryer Error":
				status.DryerError = value
			case "Dryer Warning":
			}
		}
	}
	return
}

func populateElectrolyserData(text string) (status *electrolyserStatus) {
	valueLines := getKeyValueLines(text, ": ")
	var levelLow, levelMedium, levelHigh, levelVeryHigh bool
	if len(valueLines) > 0 {
		status = new(electrolyserStatus)
		for _, valueLine := range valueLines {
			keyValue := strings.Split(valueLine, ":")
			if len(keyValue) < 2 {
				return
			}
			key := strings.Trim(keyValue[0], " ")
			value := strings.Trim(keyValue[1], " ")
			//			fmt.Println("Key =", key, "\nValue = ", value, "\nNumeric = ", strings.TrimFunc(value, notNumeric))
			switch key {
			case "Device":
				status.Device, _ = strconv.ParseInt(value, 10, 64)
			case "IP":
				status.IP = value
			case "Model":
				status.Model = value
			case "FW":
				status.Firmware = value
			case "Serial":
				status.Serial = value
			case "System state":
				status.SystemState = value
			case "H2 Flow":
				status.H2Flow, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 64)
			case "Very High Electrolyte":
				if strings.Trim(value, " ") == "Yes" {
					levelVeryHigh = true
				}
			case "High Electrolyte":
				if strings.Trim(value, " ") == "Yes" {
					levelHigh = true
				}
			case "Medium Electrolyte":
				if strings.Trim(value, " ") == "Yes" {
					levelMedium = true
				}
			case "Low Electrolyte":
				if strings.Trim(value, " ") == "Yes" {
					levelLow = true
				}
			case "Inner H2 Pressure":
				status.InnerH2Pressure, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 64)
			case "Outer H2 Pressure":
				status.OuterH2Pressure, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 64)
			case "water Pressure":
				status.WaterPressure, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 64)
			case "Electrolyte Temp":
				status.ElectrolyteTemp, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 64)
			case "Current Production Rate":
				status.CurrentProductionRate, _ = strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 64)
			case "Default Production Rate":
				status.DefaultProductionRate, _ = strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 64)
			case "Max Tank Pressure":
				status.MaxTankPressure, _ = strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 64)
			case "Restart Pressure":
				status.RestartPressure, _ = strconv.ParseInt(strings.TrimFunc(value, notNumeric), 10, 64)
			case "el state":
				status.ElState = value
			case "Stack Voltage":
				status.StackVoltage, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 64)
			case "Stack Current":
				status.StackCurrent, _ = strconv.ParseFloat(strings.TrimFunc(value, notNumeric), 64)
			}
		}
		if levelVeryHigh {
			status.ElectrolyteLevel = "Very High"
		} else if levelHigh {
			status.ElectrolyteLevel = "High"
		} else if levelMedium {
			status.ElectrolyteLevel = "Medium"
		} else if levelLow {
			status.ElectrolyteLevel = "Low"
		} else {
			status.ElectrolyteLevel = "Unknown"
		}
	} else {
		fmt.Println("No data found")
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

func getDryerStatus(device int16) (status *dryerStatus) {
	drStatus := new(dryerStatus)
	drStatus.Status = "Switched Off"
	if (device < 1) || (device > systemConfig.NumDryer) {
		log.Panic("Invalid electrolyser number - ", device)
	}

	if ((device == 1) && !SystemStatus.Relays.Electrolyser1) || ((device == 2) && !SystemStatus.Relays.Electrolyser2) {
		return drStatus
	}

	strCommand := fmt.Sprintf("dr info %d", device-1)
	text, err := sendCommand(strCommand)
	if err != nil {
		fmt.Println(err)
	}
	return populateDryerData(text)
}

func getGasStatus() {
	text, err := sendCommand("gas info")
	if err != nil {
		fmt.Println(err)
	}
	populateGasData(text)
}

func getTdsStatus() {
	text, err := sendCommand("tds info")
	if err != nil {
		fmt.Println(err)
	}
	populateTdsData(text)
}

func getElectrolyserStatus(device int16) (status *electrolyserStatus) {
	elStatus := new(electrolyserStatus)
	elStatus.SystemState = "Switched Off"

	if (device < 1) || (device > systemConfig.NumEl) {
		log.Panic("Invalid electrolyser number - ", device)
	}

	if ((device == 1) && !SystemStatus.Relays.Electrolyser1) || ((device == 2) && !SystemStatus.Relays.Electrolyser2) {
		return elStatus
	}

	cmd := fmt.Sprintf("el info %d", device-1)
	text, err := sendCommand(cmd)
	if err != nil {
		fmt.Println(err)
	}
	return populateElectrolyserData(text)
}

func getFuelCellStatus(device int16) (status *fuelCellStatus) {
	fcStatus := new(fuelCellStatus)
	fcStatus.State = "Switched Off"
	if (device < 1) || (device > systemConfig.NumFc) {
		log.Panic("Invalid fuel cell in get status - ", device)
	}
	if ((device == 1) && !SystemStatus.Relays.FuelCell1Enable) || ((device == 2) && !SystemStatus.Relays.FuelCell2Enable) {
		return fcStatus
	}

	strCommand := fmt.Sprintf("fc info %d", device-1)
	text, err := sendCommand(strCommand)
	if err != nil {
		log.Println(err)
		fcStatus.State = "(Error)"
		return fcStatus
	}
	return populateFuelCellData(text)
}

func getRelayStatus() bool {
	text, err := sendCommand("relay status")
	if err != nil {
		fmt.Println(err)
	}
	return populateRelayData(text)
}

func getElectrolyserHtmlStatus(status *electrolyserStatus) (html string) {

	if status.SystemState == "Switched Off" {
		html = `<h3 style="text-align:center">Electrolyser is switched OFF</h3`
		return
	}

	html = fmt.Sprintf(`<table>
  <tr><td class="label">Model</td><td>%s</td><td class="label">Serial Number</td><td>%s</td></tr>
  <tr><td class="label">Firmware Number</td><td>%s</td><td class="label">IP Address</td><td>%s</td></tr>
  <tr><td class="label">System State</td><td>%s</td><td class="label">Electrolyser State</td><td>%s</td></tr>
  <tr><td class="label">Electrolyte Level</td><td>%s</td><td class="label">Electrolyte Temp</td><td>%0.1f ℃</td></tr>
  <tr><td class="label">Inner H2 Pressure</td><td>%0.2f bar</td><td class="label">Outer H2 Pressure</td><td>%0.2f bar</td></tr>
  <tr><td class="label">H2 Flow</td><td>%0.2f NL/hour</td><td class="label">Water Pressure</td><td>%0.1f bar</td></tr>
  <tr><td class="label">Max Tank Pressure</td><td>%d bar</td><td class="label">Restart Pressure</td><td>%d bar</td></tr>
  <tr><td class="label">Current Production Rate</td><td>%d%%</td><td class="label">Default Production Rate</td><td>%d%%</td></tr>
  <tr><td class="label">Stack Voltage</td><td>%0.2f volts</td><td class="label">&nbsp;</td><td>&nbsp;</td></tr>
</table>`, status.Model, status.Serial, status.Firmware, status.IP,
		status.SystemState, status.ElState, status.ElectrolyteLevel, status.ElectrolyteTemp,
		status.InnerH2Pressure, status.OuterH2Pressure, status.H2Flow, status.WaterPressure,
		status.MaxTankPressure, status.RestartPressure, status.CurrentProductionRate, status.DefaultProductionRate,
		status.StackVoltage)
	return html
}

func getDryerHtmlStatus(status *dryerStatus) (html string) {
	if status.Status == "Switched Off" {
		html = `<h3 style="text-align:center">Dryer is switched OFF</h3>`
		return
	}

	html = fmt.Sprintf(`<table>
	 <tr><td class="label">Temperature 0</td><td>%0.2f℃</td><td class="label">Temperature 1</td><td>%0.2f℃</td></tr>
	 <tr><td class="label">Temperature 2</td><td>%0.2f℃</td><td class="label">Temperature 3</td><td>%0.2f℃</td></tr>
	 <tr><td class="label">Input Pressure</td><td>%0.2f bar</td><td class="label">Ouput Pressure</td><td>%0.2f bar</td></tr>
	 <tr><td class="label">Dryer Error</td><td>%s</td><td class="label">Dryer Warning</td><td>%s</td></tr>
	</table>`,
		status.DryerTemp0, status.DryerTemp1, status.DryerTemp2, status.DryerTemp3,
		status.InputPressure, status.OutputPressure, status.DryerError, status.DryerWarning)
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

func getFuelCellError(faultFlag rune, FaultFlag string) []string {
	var errors []string

	if strings.Trim(FaultFlag, " ") == "" {
		return errors
	}
	FaultFlag = strings.Trim(FaultFlag, " ")
	faultFlagValue, err := strconv.ParseUint(FaultFlag, 16, 32)
	if err != nil {
		log.Println(err)
		return errors
	}
	if faultFlagValue == 0xffffffff {
		return errors
	}
	mask := uint64(0x80000000)
	for i := 0; i < 32; i++ {
		if (faultFlagValue & mask) != 0 {
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

func getFuelCellHtmlStatus(status *fuelCellStatus) (html string) {

	if status.State == "Switched Off" {
		return `<h3 style="text-align:center">Fuel Cell is switched OFF</h3>"`
	}
	html = fmt.Sprintf(`<table>
  <tr><td class="label">Serial Number</td><td>%s</td><td class="label">Version</td><td>%s</td></tr>
  <tr><td class="label">Output Power</td><td>%0.2fW</td><td class="label">Output Volts</td><td>%0.2fV</td></tr>
  <tr><td class="label">Output Current</td><td>%0.2fA</td><td class="label">Anode Pressure</td><td>%0.2f Millibar</td></tr>
  <tr><td class="label">Inlet Temperature</td><td>%0.2f℃</td><td class="label">Outlet Temperature</td><td>%0.2f℃</td></tr>
  <tr><td class="label" colspan=2>State</td><td colspan=2>%s</td></tr>
  <tr><td class="label">Fault Flag A</td><td>%s</td><td class="label">Fault Flag B</td><td>%s</td></tr>
  <tr><td class="label">Fault Flag C</td><td>%s</td><td class="label">Fault Flag D</td><td>%s</td></tr>
</table>`, status.SerialNumber, status.Version, status.OutputPower, status.OutputVolt,
		status.OutputCurrent, status.AnodePress, status.InletTemp, status.OutletTemp,
		status.State, buildToolTip(getFuelCellError('A', status.FaultFlagA)),
		buildToolTip(getFuelCellError('B', status.FaultFlagB)),
		buildToolTip(getFuelCellError('C', status.FaultFlagC)),
		buildToolTip(getFuelCellError('D', status.FaultFlagD)))
	return html
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
		fmt.Println(w, `<head><title>Firefly Status Error</title></head>
<body><h1>ERROR feching relay status.</h1><br />
<h3>One or more relays could not be identified in the "relay status" command.</h3>
</body></html>`)
		return
	}
	_, err := fmt.Fprintf(w, `<html>
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
	</div>
    <div>
      <h2>Electrolyser 1</h2>
      %s
    </div>
    <div>
      <h2>Electrolyser 2</h2>
      %s
    </div>
    <div>
      <div style="float:left; width:48%%">
        <h2>Dryer</h2>
        %s
      </div>
      <div style="float:left; width:48%%">
        <h2>Fuel Cell</h2>
	    %s
	  </div>
      <div style="float:left; clear:both; width:48%%">
        <h2>Gas</h2>
        %s
      </div>
      <div style="float:left; width:48%%">
        <h2>TDS</h2>
        %s
      </div>
    </div>
    <div style="clear:both">
      <a href="/">Back to the Menu</a>
    </div>
  </body>
<script>
	var tID = setTimeout(function () {
		window.location.reload(true);
		window.clearTimeout(tID);		// clear time out.
	}, 5000);
</script>
</html>`,
		getRelayHtmlStatus(),
		getElectrolyserHtmlStatus(getElectrolyserStatus(1)),
		getElectrolyserHtmlStatus(getElectrolyserStatus(2)),
		getDryerHtmlStatus(getDryerStatus(1)),
		getFuelCellHtmlStatus(getFuelCellStatus(1)),
		getGasHtmlStatus(),
		getTdsHtmlStatus())
	if err != nil {
		log.Print(err)
	}
}

func errorToJson(err error) []byte {
	var errStruct struct {
		Error string
	}
	errStruct.Error = err.Error()

	jsonString, errMarshal := json.Marshal(errStruct)
	if errMarshal != nil {
		log.Print(errMarshal)
	}
	return jsonString
}

func getElectrolyserJsonStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 2) || (device < 1) {
		log.Print("Invalid electrolyser in status request")
		getStatus(w, r)
		return
	}
	str, err := json.Marshal(getElectrolyserStatus(int16(device)))
	if err != nil {
		if _, err := fmt.Fprint(w, errorToJson(err)); err != nil {
			log.Print(err)
		}
	}
	if _, err := fmt.Fprint(w, str); err != nil {
		log.Print(err)
	}
}

func getDryerJsonStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 2) || (device < 1) {
		log.Print("Invalid dryer in status request")
		getStatus(w, r)
		return
	}
	str, err := json.Marshal(getElectrolyserStatus(int16(device)))
	if err != nil {
		if _, err := fmt.Fprint(w, errorToJson(err)); err != nil {
			log.Print(err)
		}
	}
	if _, err := fmt.Fprint(w, str); err != nil {
		log.Print(err)
	}
}

func getFuelCellJsonStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 2) || (device < 1) {
		log.Print("Invalid fuel cell in status request")
		getStatus(w, r)
		return
	}
	SystemStatus.m.Lock()
	defer SystemStatus.m.Unlock()
	var status *fuelCellStatus
	if (device == 1) && SystemStatus.Relays.FuelCell1Enable {
		status = getFuelCellStatus(int16(device))
	} else {
		status = new(fuelCellStatus)
		status.State = "Switched Off"
	}
	str, err := json.Marshal(status)
	if err != nil {
		if _, err := fmt.Fprint(w, errorToJson(err)); err != nil {
			log.Print(err)
		}
	}
	if _, err := fmt.Fprint(w, str); err != nil {
		log.Print(err)
	}
}

func getSystemStatus() {
	if !getRelayStatus() {
		SystemStatus.valid = false
		return
	}
	SystemStatus.m.Lock()
	defer SystemStatus.m.Unlock()

	getGasStatus()
	getTdsStatus()
	SystemStatus.FuelCells = nil
	for fc := int16(1); fc <= systemConfig.NumFc; fc++ {
		SystemStatus.FuelCells = append(SystemStatus.FuelCells, getFuelCellStatus(fc))
	}
	SystemStatus.Electrolysers = nil
	for el := int16(1); el <= systemConfig.NumEl; el++ {
		SystemStatus.Electrolysers = append(SystemStatus.Electrolysers, getElectrolyserStatus(el))
	}
	SystemStatus.Dryers = nil
	for dr := int16(1); dr <= systemConfig.NumDryer; dr++ {
		SystemStatus.Dryers = append(SystemStatus.Dryers, getDryerStatus(dr))
	}
	SystemStatus.valid = true
}

func logStatus() {
	var err error
	if pDB == nil {
		if pDB, err = connectToDatabase(); err != nil {
			log.Print(err)
			return
		}
	}

	var params struct {
		el1Status           sql.NullString
		el1Rate             sql.NullInt64
		el1ElectrolyteLevel sql.NullString
		el1ElectrolyteTemp  sql.NullFloat64
		el1State            sql.NullString
		el1H2Flow           sql.NullFloat64
		el1H2InnerPressure  sql.NullFloat64
		el1H2OuterPressure  sql.NullFloat64
		el1StackVoltage     sql.NullFloat64
		el1StackCurrent     sql.NullFloat64
		el1SystemState      sql.NullString
		el1WaterPressure    sql.NullFloat64

		dr1Status         sql.NullString
		dr1Temp0          sql.NullFloat64
		dr1Temp1          sql.NullFloat64
		dr1Temp2          sql.NullFloat64
		dr1Temp3          sql.NullFloat64
		dr1InputPressure  sql.NullFloat64
		dr1OutputPressure sql.NullFloat64
		dr1Warning        sql.NullString

		el2Status           sql.NullString
		el2Rate             sql.NullInt64
		el2ElectrolyteLevel sql.NullString
		el2ElectrolyteTemp  sql.NullFloat64
		el2State            sql.NullString
		el2H2Flow           sql.NullFloat64
		el2H2InnerPressure  sql.NullFloat64
		el2H2OuterPressure  sql.NullFloat64
		el2StackVoltage     sql.NullFloat64
		el2StackCurrent     sql.NullFloat64
		el2SystemState      sql.NullString
		el2WaterPressure    sql.NullFloat64

		dr2Status         sql.NullString
		dr2Temp0          sql.NullFloat64
		dr2Temp1          sql.NullFloat64
		dr2Temp2          sql.NullFloat64
		dr2Temp3          sql.NullFloat64
		dr2InputPressure  sql.NullFloat64
		dr2OutputPressure sql.NullFloat64
		dr2Warning        sql.NullString

		fc1State         sql.NullString
		fc1AnodePressure sql.NullFloat64
		fc1FaultFlagA    sql.NullString
		fc1FaultFlagB    sql.NullString
		fc1FaultFlagC    sql.NullString
		fc1FaultFlagD    sql.NullString
		fc1InletTemp     sql.NullFloat64
		fc1OutletTemp    sql.NullFloat64
		fc1OutputPower   sql.NullFloat64
		fc1OutputCurrent sql.NullFloat64
		fc1OutputVoltage sql.NullFloat64

		fc2State         sql.NullString
		fc2AnodePressure sql.NullFloat64
		fc2FaultFlagA    sql.NullString
		fc2FaultFlagB    sql.NullString
		fc2FaultFlagC    sql.NullString
		fc2FaultFlagD    sql.NullString
		fc2InletTemp     sql.NullFloat64
		fc2OutletTemp    sql.NullFloat64
		fc2OutputPower   sql.NullFloat64
		fc2OutputCurrent sql.NullFloat64
		fc2OutputVoltage sql.NullFloat64
	}

	SystemStatus.m.Lock()
	defer SystemStatus.m.Unlock()

	params.el1Status.Valid = false
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

	params.el2Status.Valid = false
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

	params.dr1Status.Valid = false
	params.dr1Temp0.Valid = false
	params.dr1Temp1.Valid = false
	params.dr1Temp2.Valid = false
	params.dr1Temp3.Valid = false
	params.dr1InputPressure.Valid = false
	params.dr1OutputPressure.Valid = false
	params.dr1Warning.Valid = false

	params.dr2Status.Valid = false
	params.dr2Temp0.Valid = false
	params.dr2Temp1.Valid = false
	params.dr2Temp2.Valid = false
	params.dr2Temp3.Valid = false
	params.dr2InputPressure.Valid = false
	params.dr2OutputPressure.Valid = false
	params.dr2Warning.Valid = false

	params.fc1State.Valid = false
	params.fc1AnodePressure.Valid = false
	params.fc1FaultFlagA.Valid = false
	params.fc1FaultFlagB.Valid = false
	params.fc1FaultFlagC.Valid = false
	params.fc1FaultFlagD.Valid = false
	params.fc1InletTemp.Valid = false
	params.fc1OutletTemp.Valid = false
	params.fc1OutputPower.Valid = false
	params.fc1OutputCurrent.Valid = false
	params.fc1OutputVoltage.Valid = false

	params.fc2State.Valid = false
	params.fc2AnodePressure.Valid = false
	params.fc2FaultFlagA.Valid = false
	params.fc2FaultFlagB.Valid = false
	params.fc2FaultFlagC.Valid = false
	params.fc2FaultFlagD.Valid = false
	params.fc2InletTemp.Valid = false
	params.fc2OutletTemp.Valid = false
	params.fc2OutputPower.Valid = false
	params.fc2OutputCurrent.Valid = false
	params.fc2OutputVoltage.Valid = false

	if len(SystemStatus.Electrolysers) > 0 {
		if SystemStatus.Relays.Electrolyser1 {
			params.el1SystemState.String = SystemStatus.Electrolysers[0].SystemState
			params.el1SystemState.Valid = true
			params.el1ElectrolyteLevel.String = SystemStatus.Electrolysers[0].ElectrolyteLevel
			params.el1ElectrolyteLevel.Valid = true
			params.el1H2Flow.Float64 = SystemStatus.Electrolysers[0].H2Flow
			params.el1H2Flow.Valid = true
			params.el1ElectrolyteTemp.Float64 = SystemStatus.Electrolysers[0].ElectrolyteTemp
			params.el1ElectrolyteTemp.Valid = true
			params.el1Status.String = SystemStatus.Electrolysers[0].ElState
			params.el1Status.Valid = true
			params.el1State.String = SystemStatus.Electrolysers[0].SystemState
			params.el1State.Valid = true
			params.el1H2InnerPressure.Float64 = SystemStatus.Electrolysers[0].InnerH2Pressure
			params.el1H2InnerPressure.Valid = true
			params.el1H2OuterPressure.Float64 = SystemStatus.Electrolysers[0].OuterH2Pressure
			params.el1H2OuterPressure.Valid = true
			params.el1Rate.Int64 = SystemStatus.Electrolysers[0].CurrentProductionRate
			params.el1Rate.Valid = true
			params.el1StackVoltage.Float64 = SystemStatus.Electrolysers[0].StackVoltage
			params.el1StackVoltage.Valid = true
			params.el1StackCurrent.Float64 = SystemStatus.Electrolysers[0].StackCurrent
			params.el1StackCurrent.Valid = true
			params.el1WaterPressure.Float64 = SystemStatus.Electrolysers[0].WaterPressure
			params.el1WaterPressure.Valid = true
		} else {
			params.el1SystemState.String = "Powered Down"
			params.el1SystemState.Valid = true
		}
	}
	if len(SystemStatus.Electrolysers) > 1 {
		if SystemStatus.Relays.Electrolyser2 {
			params.el2ElectrolyteLevel.String = SystemStatus.Electrolysers[1].ElectrolyteLevel
			params.el2ElectrolyteLevel.Valid = true
			params.el2H2Flow.Float64 = SystemStatus.Electrolysers[1].H2Flow
			params.el2H2Flow.Valid = true
			params.el2ElectrolyteTemp.Float64 = SystemStatus.Electrolysers[1].ElectrolyteTemp
			params.el2ElectrolyteTemp.Valid = true
			params.el2Status.String = SystemStatus.Electrolysers[1].ElState
			params.el2Status.Valid = true
			params.el2State.String = SystemStatus.Electrolysers[1].SystemState
			params.el2State.Valid = true
			params.el2H2InnerPressure.Float64 = SystemStatus.Electrolysers[1].InnerH2Pressure
			params.el2H2InnerPressure.Valid = true
			params.el2H2OuterPressure.Float64 = SystemStatus.Electrolysers[1].OuterH2Pressure
			params.el2H2OuterPressure.Valid = true
			params.el2Rate.Int64 = SystemStatus.Electrolysers[1].CurrentProductionRate
			params.el2Rate.Valid = true
			params.el2StackVoltage.Float64 = SystemStatus.Electrolysers[1].StackVoltage
			params.el2StackVoltage.Valid = true
			params.el2StackCurrent.Float64 = SystemStatus.Electrolysers[1].StackCurrent
			params.el2StackCurrent.Valid = true
			params.el2WaterPressure.Float64 = SystemStatus.Electrolysers[1].WaterPressure
			params.el2WaterPressure.Valid = true
			params.el2SystemState.String = SystemStatus.Electrolysers[1].SystemState
		} else {
			params.el2SystemState.String = "Powered Down"
			params.el2SystemState.Valid = true
		}
	}
	if len(SystemStatus.Dryers) > 0 {
		if SystemStatus.Relays.Electrolyser1 {
			params.dr1Status.String = SystemStatus.Dryers[0].Status
			params.dr1Status.Valid = true
			params.dr1InputPressure.Float64 = SystemStatus.Dryers[0].InputPressure
			params.dr1InputPressure.Valid = true
			params.dr1OutputPressure.Float64 = SystemStatus.Dryers[0].OutputPressure
			params.dr1OutputPressure.Valid = true
			params.dr1Temp0.Float64 = SystemStatus.Dryers[0].DryerTemp0
			params.dr1Temp0.Valid = true
			params.dr1Temp1.Float64 = SystemStatus.Dryers[0].DryerTemp1
			params.dr1Temp1.Valid = true
			params.dr1Temp2.Float64 = SystemStatus.Dryers[0].DryerTemp2
			params.dr1Temp2.Valid = true
			params.dr1Temp3.Float64 = SystemStatus.Dryers[0].DryerTemp3
			params.dr1Temp3.Valid = true
			params.dr1Warning.String = SystemStatus.Dryers[0].DryerWarning
			params.dr1Warning.Valid = true
		} else {
			params.dr1Status.String = "Powered Down"
			params.dr1Status.Valid = true
		}
	}
	if len(SystemStatus.Dryers) > 1 {
		if SystemStatus.Relays.Electrolyser2 {
			params.dr2Status.String = SystemStatus.Dryers[1].Status
			params.dr2Status.Valid = true
			params.dr2InputPressure.Float64 = SystemStatus.Dryers[1].InputPressure
			params.dr2InputPressure.Valid = true
			params.dr2OutputPressure.Float64 = SystemStatus.Dryers[1].OutputPressure
			params.dr2OutputPressure.Valid = true
			params.dr2Temp0.Float64 = SystemStatus.Dryers[1].DryerTemp0
			params.dr2Temp0.Valid = true
			params.dr2Temp1.Float64 = SystemStatus.Dryers[1].DryerTemp1
			params.dr2Temp1.Valid = true
			params.dr2Temp2.Float64 = SystemStatus.Dryers[1].DryerTemp2
			params.dr2Temp2.Valid = true
			params.dr2Temp3.Float64 = SystemStatus.Dryers[1].DryerTemp3
			params.dr2Temp3.Valid = true
			params.dr2Warning.String = SystemStatus.Dryers[1].DryerWarning
			params.dr2Warning.Valid = true
		} else {
			params.dr2Status.String = "Powered Down"
			params.dr2Status.Valid = true
		}
	}
	if len(SystemStatus.FuelCells) > 0 {
		if SystemStatus.Relays.FuelCell1Enable {
			params.fc1AnodePressure.Float64 = SystemStatus.FuelCells[0].AnodePress
			params.fc1AnodePressure.Valid = true
			params.fc1FaultFlagA.String = SystemStatus.FuelCells[0].FaultFlagA
			params.fc1FaultFlagA.Valid = true
			params.fc1FaultFlagB.String = SystemStatus.FuelCells[0].FaultFlagB
			params.fc1FaultFlagB.Valid = true
			params.fc1FaultFlagC.String = SystemStatus.FuelCells[0].FaultFlagC
			params.fc1FaultFlagC.Valid = true
			params.fc1FaultFlagD.String = SystemStatus.FuelCells[0].FaultFlagD
			params.fc1FaultFlagD.Valid = true
			params.fc1InletTemp.Float64 = SystemStatus.FuelCells[0].InletTemp
			params.fc1InletTemp.Valid = true
			params.fc1OutletTemp.Float64 = SystemStatus.FuelCells[0].OutletTemp
			params.fc1OutletTemp.Valid = true
			params.fc1OutputCurrent.Float64 = SystemStatus.FuelCells[0].OutputCurrent
			params.fc1OutputCurrent.Valid = true
			params.fc1OutputVoltage.Float64 = SystemStatus.FuelCells[0].OutputVolt
			params.fc1OutputVoltage.Valid = true
			params.fc1OutputPower.Float64 = SystemStatus.FuelCells[0].OutputPower
			params.fc1OutputPower.Valid = true
			params.fc1State.String = SystemStatus.FuelCells[0].State
			params.fc1State.Valid = true
		} else {
			params.fc1State.String = "Powered Down"
			params.fc1State.Valid = true
		}
	}
	if len(SystemStatus.FuelCells) > 1 {
		if SystemStatus.Relays.FuelCell1Enable {
			params.fc2AnodePressure.Float64 = SystemStatus.FuelCells[1].AnodePress
			params.fc2AnodePressure.Valid = true
			params.fc2FaultFlagA.String = SystemStatus.FuelCells[1].FaultFlagA
			params.fc2FaultFlagA.Valid = true
			params.fc2FaultFlagB.String = SystemStatus.FuelCells[1].FaultFlagB
			params.fc2FaultFlagB.Valid = true
			params.fc2FaultFlagC.String = SystemStatus.FuelCells[1].FaultFlagC
			params.fc2FaultFlagC.Valid = true
			params.fc2FaultFlagD.String = SystemStatus.FuelCells[1].FaultFlagD
			params.fc2FaultFlagD.Valid = true
			params.fc2InletTemp.Float64 = SystemStatus.FuelCells[1].InletTemp
			params.fc2InletTemp.Valid = true
			params.fc2OutletTemp.Float64 = SystemStatus.FuelCells[1].OutletTemp
			params.fc2OutletTemp.Valid = true
			params.fc2OutputCurrent.Float64 = SystemStatus.FuelCells[1].OutputCurrent
			params.fc2OutputCurrent.Valid = true
			params.fc2OutputVoltage.Float64 = SystemStatus.FuelCells[1].OutputVolt
			params.fc2OutputVoltage.Valid = true
			params.fc2OutputPower.Float64 = SystemStatus.FuelCells[1].OutputPower
			params.fc2OutputPower.Valid = true
			params.fc2State.String = SystemStatus.FuelCells[1].State
			params.fc2State.Valid = true
		} else {
			params.fc2State.String = "Powered  Down"
			params.fc2State.Valid = true
		}
	}

	strCommand := `INSERT INTO firefly.logging(
            el1Status, el1Rate, el1ElectrolyteLevel, el1ElectrolyteTemp, el1State, el1H2Flow, el1H2InnerPressure, el1H2OuterPressure, el1StackVoltage, el1StackCurrent, el1SystemState, el1WaterPressure, 
            dr1Status, dr1Temp0, dr1Temp1, dr1Temp2, dr1Temp3, dr1InputPressure, dr1OutputPressure, dr1Warning, 
            el2Status, el2Rate, el2ElectrolyteLevel, el2ElectrolyteTemp, el2State, el2H2Flow, el2H2InnerPressure, el2H2OuterPressure, el2StackVoltage, el2StackCurrent, el2SystemState, el2WaterPressure,
            dr2Status, dr2Temp0, dr2Temp1, dr2Temp2, dr2Temp3, dr2InputPressure, dr2OutputPressure, dr2Warning,
            fc1State, fc1AnodePressure, fc1FaultFlagA, fc1FaultFlagB, fc1FaultFlagC, fc1FaultFlagD, fc1InletTemp, fc1OutletTemp, fc1OutputPower, fc1OutputCurrent, fc1OutputVoltage,
            fc2State, fc2AnodePressure, fc2FaultFlagA, fc2FaultFlagB, fc2FaultFlagC, fc2FaultFlagD, fc2InletTemp, fc2OutletTemp, fc2OutputPower, fc2OutputCurrent, fc2OutputVoltage,
            gasFuelCellPressure, gasTankPressure,
            totalDissolvedSolids,
            relayGas, relayFuelCell1Enable, relayFuelCell1Run, relayFuelCell2Enable, relayFuelCell2Run, relayEl1Power, relayEl2Power, relayDrain)
	VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
	       ?, ?, ?, ?, ?, ?, ?, ?,
	       ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
	       ?, ?, ?, ?, ?, ?, ?, ?,
	       ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
	       ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
	       ?, ?,
	       ?,
	       ?, ?, ?, ?, ?, ?, ?, ?);`

	_, err = pDB.Exec(strCommand,
		params.el1Status, params.el1Rate, params.el1ElectrolyteLevel, params.el1ElectrolyteTemp, params.el1Status, params.el1H2Flow, params.el1H2InnerPressure, params.el1H2OuterPressure, params.el1StackVoltage, params.el1StackCurrent, params.el1SystemState, params.el1WaterPressure,
		params.dr1Status, params.dr1Temp0, params.dr1Temp1, params.dr1Temp2, params.dr1Temp3, params.dr1InputPressure, params.dr1OutputPressure, params.dr1Warning,
		params.el2Status, params.el2Rate, params.el2ElectrolyteLevel, params.el2ElectrolyteTemp, params.el2Status, params.el2H2Flow, params.el2H2InnerPressure, params.el2H2OuterPressure, params.el2StackVoltage, params.el2StackCurrent, params.el2SystemState, params.el2WaterPressure,
		params.dr2Status, params.dr2Temp0, params.dr2Temp1, params.dr2Temp2, params.dr2Temp3, params.dr2InputPressure, params.dr2OutputPressure, params.dr2Warning,
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

//func getJsonStatus() string {
//	getSystemStatus()
//	strStatus, err := json.Marshal(&SystemStatus)
//	if err != nil {
//		log.Println(err)
//		return err.Error()
//	}
//	return string(strStatus)
//}

func getMinJsonStatus() string {
	type minElectrolyserStatus struct {
		State string
		Rate  int8
		Flow  float32
	}
	type minFuelCellStatus struct {
		State  string
		Output float32
	}
	var minStatus struct {
		Electrolysers []*minElectrolyserStatus
		FuelCells     []*minFuelCellStatus
		Gas           float32
	}
	minStatus.Gas = float32(SystemStatus.Gas.TankPressure)
	for _, el := range SystemStatus.Electrolysers {
		minEl := new(minElectrolyserStatus)
		minEl.Rate = int8(el.CurrentProductionRate)
		minEl.State = strings.Replace(el.ElState, "EL_STATE_", "", -1)
		minEl.Flow = float32(el.H2Flow)
		minStatus.Electrolysers = append(minStatus.Electrolysers, minEl)
	}
	for _, fc := range SystemStatus.FuelCells {
		minFc := new(minFuelCellStatus)
		minFc.State = fc.State
		minFc.Output = float32(fc.OutputPower)
		minStatus.FuelCells = append(minStatus.FuelCells, minFc)
	}
	if status, err := json.Marshal(minStatus); err != nil {
		log.Println(err)
		return ""
	} else {
		return string(status)
	}
}

func getMinHtmlStatus(w http.ResponseWriter, _ *http.Request) {
	if _, err := fmt.Fprint(w, getMinJsonStatus()); err != nil {
		log.Println(err)
	}
}

func getElectrolyserHistory(w http.ResponseWriter, r *http.Request) {
	type Row struct {
		Logged            string
		EL1Rate           float64
		EL1Temp           float64
		EL1State          string
		EL1H2Flow         float64
		EL1InnerPressure  float64
		EL1OuterPressure  float64
		EL1StackVoltage   float64
		EL1StackCurrent   float64
		EL1SystemState    string
		EL1WaterPressure  float64
		DR1Temp0          float64
		DR1Temp1          float64
		DR1Temp2          float64
		DR1Temp3          float64
		DR1InputPressure  float64
		DR1OutputPressure float64
		EL2Rate           float64
		EL2Temp           float64
		EL2State          string
		EL2H2Flow         float64
		EL2InnerPressure  float64
		EL2OuterPressure  float64
		EL2StackVoltage   float64
		EL2StackCurrent   float64
		EL2SystemState    string
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

	vars := mux.Vars(r)
	from := vars["from"]
	to := vars["to"]
	rows, err := pDB.Query(`SELECT (UNIX_TIMESTAMP(logged) DIV 60) * 60, IFNULL(AVG(el1Rate) ,0), IFNULL(AVG(el1ElectrolyteTemp) ,0), IFNULL(MAX(el1State) ,''), IFNULL(AVG(el1H2Flow), 0), IFNULL(AVG(el1H2InnerPressure), 0),
		IFNULL(AVG(el1H2OuterPressure), 0), IFNULL(AVG(el1StackVoltage), 0), IFNULL(AVG(el1StackCurrent), 0), IFNULL(MAX(el1SystemState), ''), IFNULL(AVG(el1WaterPressure), 0),
		IFNULL(AVG(dr1Temp0), 0), IFNULL(AVG(dr1Temp1), 0), IFNULL(AVG(dr1Temp2), 0), IFNULL(AVG(dr1Temp3), 0), IFNULL(AVG(dr1InputPressure), 0), IFNULL(AVG(dr1OutputPressure), 0),
		IFNULL(AVG(el2Rate), 0), IFNULL(AVG(el2ElectrolyteTemp), 0), IFNULL(MAX(el2State), ''), IFNULL(AVG(el2H2Flow), 0), IFNULL(AVG(el2H2InnerPressure), 0),
		IFNULL(AVG(el2H2OuterPressure), 0), IFNULL(AVG(el2StackVoltage), 0), IFNULL(AVG(el2StackCurrent), 0), IFNULL(MAX(el2SystemState), ''), IFNULL(AVG(el2WaterPressure), 0),
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
			log.Println(err)
		}
	} else {
		if _, err := fmt.Fprintf(w, string(JSON)); err != nil {
			log.Println(err)
		}
	}
}

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

	vars := mux.Vars(r)
	from := vars["from"]
	to := vars["to"]

	log.Println("From ", from, " to ", to)

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

func setElOn(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device := vars["device"]
	var strCommand string
	switch device {
	case "1":
		strCommand = "el1dr on"
	case "2":
		strCommand = "el2 on"
	default:
		log.Print("Invalid electrolyser specified - ", device)
		getStatus(w, r)
		return
	}
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
    <title>Firefly Electrolyser On</title>
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

func setElOff(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device := vars["device"]
	var strCommand string
	switch device {
	case "1":
		strCommand = "el1dr off"
	case "2":
		strCommand = "el2 off"
	default:
		log.Print("Invalid electrolyser specified - ", device)
		getStatus(w, r)
		return
	}
	response, err := sendCommand(strCommand)
	_, err = fmt.Fprintf(w, `<html>
  <head>
    <title>Firefly Electrolyser Off</title>
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

func setElStart(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 2) || (device < 1) {
		log.Print("Invalid electrolyser in start request")
		getStatus(w, r)
		return
	}
	strCommand := fmt.Sprintf("el start %d", device-1)

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
    <title>Firefly Electrolyser Start</title>
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

func setElStop(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 2) || (device < 1) {
		log.Print("Invalid electrolyser in set production rate request")
		getStatus(w, r)
		return
	}
	strCommand := fmt.Sprintf("el stop %d", device-1)
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
    <title>Firefly Electrolyser Stop</title>
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

func setElSetRate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 2) || (device < 1) {
		log.Print("Invalid electrolyser in set production rate request")
		getStatus(w, r)
		return
	}
	//if b, err := io.ReadAll(r.Body); err != nil {
	//	fmt.Println((err))
	//} else {
	//	fmt.Println(string(b))
	//}

	if err := r.ParseForm(); err != nil {
		log.Print(err)
		getStatus(w, r)
		return
	}
	rateVal := r.FormValue("rate")
	rate, err := strconv.ParseInt(rateVal, 10, 16)
	if err != nil {
		log.Print(err)
		getStatus(w, r)
		return
	}
	strCommand := ""
	if (rate > 59) && (rate < 101) {
		if SystemStatus.Electrolysers[device-1].ElState == ElIdle {
			strCommand = fmt.Sprintf("el start %d", device-1)

			_, err := sendCommand(strCommand)
			if err != nil {
				log.Print(err)
				_, err = fmt.Fprintf(w, err.Error())
				if err != nil {
					log.Print(err)
				}
				return
			}

		}
		strCommand = fmt.Sprintf("el set %d pr %d", device-1, rate)
	} else if rate == 0 {
		strCommand = fmt.Sprintf("el stop %d", device-1)
	} else {
		log.Printf("Invalid production rate (%d) requested for electrolyser %d", rate, device)
		getStatus(w, r)
	}
	response, err := sendCommand(strCommand)
	if err != nil {
		log.Print(err)
		_, err = fmt.Fprintf(w, err.Error())
		if err != nil {
			log.Print(err)
		}
		getStatus(w, r)
		return
	}
	_, err = fmt.Fprintf(w, `<html>
  <head>
    <title>Firefly Electrolyser 1 Stop</title>
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

func setElReboot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 2) || (device < 1) {
		log.Print("Invalid electrolyser in reboot request")
		getStatus(w, r)
		return
	}
	strCommand := fmt.Sprintf("el reboot %d", device-1)
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
    <title>Firefly Electrolyser 1 Stop</title>
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

func setFcOn(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Set fuel cell on")
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

func setFcOff(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 2) || (device < 1) {
		log.Print("Invalid fuel cell in 'off' request")
		getStatus(w, r)
		return
	}
	strCommand := fmt.Sprintf("fc off %d", device-1)
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
    <title>Firefly Fuel Cell Off</title>
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

func setFcRun(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 2) || (device < 1) {
		log.Print("Invalid fuel cell in 'run' request")
		getStatus(w, r)
		return
	}
	strCommand := fmt.Sprintf("fc run %d", device-1)
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
    <title>Firefly Fuel Cell Start</title>
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

func setFcStop(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 2) || (device < 1) {
		log.Print("Invalid fuel cell in 'on' request")
		getStatus(w, r)
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

var upgrader = websocket.Upgrader{
	ReadBufferSize:    1024,
	WriteBufferSize:   1024,
	EnableCompression: true,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

/**
Start the Web Socket server. This sends out data to all subscribers on a regular schedule so subscribers don't need to poll for updates.
*/
func startDataWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	for {
		signal.L.Lock()   // Get the signal and lock it.
		signal.Wait()     // Wait for it to be signalled again. It is unlocked while we wait then locked again before returning
		signal.L.Unlock() // Unlock it
		w, err := conn.NextWriter(websocket.TextMessage)
		if err != nil {
			log.Println("Failed to get the values websocket writer - ", err)
			return
		}
		var sJSON = getMinJsonStatus()
		_, err = fmt.Fprint(w, sJSON)
		if err != nil {
			log.Println("failed to write the values message to the websocket - ", err)
			return
		}
		if err := w.Close(); err != nil {
			log.Println("Failed to close the values websocket writer - ", err)
		}
	}
}

func setElectrolyserRatePercent(rate uint8, device uint8) error {
	if rate > 0 {
		if SystemStatus.Electrolysers[device-1].ElState == ElIdle {
			log.Println("Starting Electrolyser ", device)
			strCommand := fmt.Sprintf("el start %d", device-1)
			_, err := sendCommand(strCommand)
			if err != nil {
				log.Print(err)
				return err
			}
		}
		strCommand := fmt.Sprintf("el set %d pr %d", device-1, rate)
		_, err := sendCommand(strCommand)
		if err != nil {
			log.Print(err)
			return err
		}
	} else {
		strCommand := fmt.Sprintf("el stop %d", device-1)
		_, err := sendCommand(strCommand)
		if err != nil {
			log.Print(err)
			return err
		}
	}
	return nil
}

func setElectrolyserRate(w http.ResponseWriter, r *http.Request) {
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

	// Return bad request if outside acceptable range of 0..100%
	if jRate.Rate > 100 || jRate.Rate < 0 {
		var jErr JSONError
		jErr.AddErrorString("electrolyser", "Rate must be between 0 and 100")
		jErr.ReturnError(w, 400)
		return
	}

	var el1, el2 uint8

	ratePercent := uint8((jRate.Rate * 122) / 100)
	switch {
	case ratePercent == 0:
		el1 = 0
		el2 = 0
	case ratePercent < 42:
		el1 = ratePercent + 59
		el2 = 0
	case ratePercent < 83:
		el1 = ratePercent + 18
		el2 = 60
	default:
		el1 = 100
		el2 = ratePercent - 22
	}

	log.Println("Set Electrolyser 1 to ", el1)
	err = setElectrolyserRatePercent(el1, 1)
	var jError JSONError
	if err != nil {
		jError.AddError("el1", err)
	}
	log.Println("Set Electrolyser 2 to ", el2)
	err = setElectrolyserRatePercent(el2, 2)
	if err != nil {
		jError.AddError("el2", err)
	}
	if len(jError.Errors) > 0 {
		jError.ReturnError(w, 500)
	}
}

func getElectrolyserRate(w http.ResponseWriter, _ *http.Request) {

	var el1, el2 float64

	if SystemStatus.Electrolysers[0].ElState == ElIdle {
		el1 = 0.0
	} else {
		el1 = float64(SystemStatus.Electrolysers[0].CurrentProductionRate)
	}
	if SystemStatus.Electrolysers[1].ElState == ElIdle {
		el2 = 0
	} else {
		el2 = float64(SystemStatus.Electrolysers[1].CurrentProductionRate)
	}
	rate := 0.0
	if el1+el2 > 0 {
		// ROUND((((A3+B3)-59)/14)*9.9, 0)
		rate = math.Round((((el1 + el2) - 59) / 14) * 9.9)
	}
	if _, err := fmt.Fprintf(w, `{"rate":%d}`, int8(rate)); err != nil {
		var jErr JSONError
		jErr.AddError("Electrolyser", err)
		jErr.ReturnError(w, 500)
	}
}

func setUpWebSite() {
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/ws", startDataWebSocket).Methods("GET")
	router.HandleFunc("/status", getStatus).Methods("GET")
	router.HandleFunc("/el/{device}/on", setElOn).Methods("GET", "POST")
	router.HandleFunc("/el/{device}/off", setElOff).Methods("GET", "POST")
	router.HandleFunc("/el/{device}/start", setElStart).Methods("GET", "POST")
	router.HandleFunc("/el/{device}/stop", setElStop).Methods("GET", "POST")
	router.HandleFunc("/el/{device}/rate", setElSetRate).Methods("POST")
	router.HandleFunc("/fc/{device}/off", setFcOff).Methods("GET", "POST")
	router.HandleFunc("/fc/{device}/on", setFcOn).Methods("GET", "POST")
	router.HandleFunc("/gas/off", setGasOff).Methods("GET", "POST")
	router.HandleFunc("/gas/on", setGasOn).Methods("GET", "POST")
	router.HandleFunc("/fc/{device}/run", setFcRun).Methods("GET", "POST")
	router.HandleFunc("/fc/{device}/stop", setFcStop).Methods("GET", "POST")
	router.HandleFunc("/el/{device}/setRate", showElectrolyserProductionRatePage).Methods("GET")
	router.HandleFunc("/el/{device}/status", getElectrolyserJsonStatus).Methods("GET")
	router.HandleFunc("/dr/{device}/status", getDryerJsonStatus).Methods("GET")
	router.HandleFunc("/fc/{device}/status", getFuelCellJsonStatus).Methods("GET")
	router.HandleFunc("/el/{device}/reboot", setElReboot).Methods("POST")
	router.HandleFunc("/minStatus", getMinHtmlStatus).Methods("GET")
	router.HandleFunc("/fcdata/{from}/{to}", getFuelCellHistory).Methods("GET")
	router.HandleFunc("/eldata/{from}/{to}", getElectrolyserHistory).Methods("GET")
	router.HandleFunc("/el/setrate", setElectrolyserRate).Methods("POST")
	router.HandleFunc("/el/getRate", getElectrolyserRate).Methods("GET")

	fileServer := http.FileServer(neuteredFileSystem{http.Dir("./web")})
	router.PathPrefix("/").Handler(http.StripPrefix("/", fileServer))

	log.Fatal(http.ListenAndServe(":20080", router))
}

func commandResponseReader(outPipe *bufio.Reader) {
	for {
		text, err := outPipe.ReadString('>')
		if err != nil {
			_, err = fmt.Println("CommandResponseReader error - ", err)
			if err != nil {
				log.Print(err)
			}
		}
		if strings.Trim(text, " ") != string(esmPrompt) {
			commandResponse <- text
		}
	}
}

func init() {

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
	flag.Parse()
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	commandResponse = make(chan string)
	esmCommand.valid = false
	var err error

	if pDB, err = connectToDatabase(); err != nil {
		log.Println(`Cannot connect to the database - `, err)
	}

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

	loops := 0
	for {
		if loops == 0 {
			getSystemStatus()
			if SystemStatus.valid {
				logStatus()
			}
		}
		signal.Broadcast()
		time.Sleep(time.Second)
		loops++
		if loops >= 10 {
			loops = 0
		}
	}
}
