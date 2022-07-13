package main

import (
	"database/sql"
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
</table>`, status.getSerial(), status.Software.Version, status.Software.Major, status.Software.Minor,
		status.OutputPower, status.OutputVolts, status.OutputCurrent,
		status.getAnodePressure(), status.getInletTemp(), status.getOutletTemp(), status.GetState(),
		buildToolTip(getFuelCellError('A', status.getFaultA())),
		buildToolTip(getFuelCellError('B', status.getFaultB())),
		buildToolTip(getFuelCellError('C', status.getFaultC())),
		buildToolTip(getFuelCellError('D', status.getFaultD())))
	return html
}

func logFuelCellData() {
	var data struct {
		Cell              sql.NullInt32
		AnodePressure     sql.NullInt32
		FaultA            uint32
		FaultB            uint32
		FaultC            uint32
		FaultD            uint32
		InletTemp         sql.NullInt32
		OutletTemp        sql.NullInt32
		Power             sql.NullInt32
		Amps              sql.NullInt32
		Volts             sql.NullInt32
		State             sql.NullString
		Fault             bool
		Run               bool
		Inactive          bool
		Standby           bool
		DCDCEnabled       bool
		DCDCDisabled      bool
		OnLoad            bool
		FanPulse          bool
		Derated           bool
		SV01              bool
		SV02              bool
		SV04              bool
		LouverOpen        bool
		DCDC_Enable       bool
		PowerFromStack    bool
		PowerFromExternal bool
	}
	strCommand := `INSERT INTO firefly.FuelCell
	(AnodePressure, Power, FaultA, FaultB, FaultC, FaultD, OutletTemp, InletTemp, Volts, Amps, Cell, Fault, Run, Inactive, Standby, DCDC_Disabled, OnLoad
	, FanPulse, Derated, SV01, SV02, SV04, LouverOpen, DCDC_Enabled, PowerFromStack, PowerFromExternal)
	VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`

	var err error
	if pDB == nil {
		if pDB, err = connectToDatabase(); err != nil {
			log.Print(err)
			return
		}
	}

	for device, fuelCell := range canBus.fuelCell {
		if fuelCell.IsSwitchedOn() {
			data.Cell.Int32 = int32(device)
			data.Cell.Valid = true
			data.AnodePressure.Int32 = int32(fuelCell.getAnodePressure() * 1000)
			data.AnodePressure.Valid = true
			data.FaultA = fuelCell.getFaultA()
			data.FaultB = fuelCell.getFaultB()
			data.FaultC = fuelCell.getFaultC()
			data.FaultD = fuelCell.getFaultD()
			data.InletTemp.Int32 = int32(fuelCell.getInletTemp() * 10)
			data.InletTemp.Valid = true
			data.OutletTemp.Int32 = int32(fuelCell.getOutletTemp() * 10)
			data.OutletTemp.Valid = true
			data.Amps.Int32 = int32(fuelCell.getOutputCurrent() * 100)
			data.Amps.Valid = true
			data.Volts.Int32 = int32(fuelCell.getOutputVolts() * 10)
			data.Volts.Valid = true
			data.Power.Int32 = int32(fuelCell.getOutputPower())
			data.Power.Valid = true
			data.State.String = fuelCell.GetState()
			data.State.Valid = true
			data.Fault = fuelCell.getFault()
			data.Run = fuelCell.getRun()
			data.Inactive = fuelCell.getInactive()
			data.Standby = fuelCell.getStandby()
			data.DCDCEnabled = fuelCell.getDCDCEnabled()
			data.DCDCDisabled = fuelCell.getDCDCDisabled()
			data.OnLoad = fuelCell.getOnLoad()
			data.FanPulse = fuelCell.getFanPulse()
			data.Derated = fuelCell.getDerated()
			data.SV01 = fuelCell.getSV01()
			data.SV02 = fuelCell.getSV02()
			data.SV04 = fuelCell.getSV04()
			data.LouverOpen = fuelCell.getLouverOpen()
			data.PowerFromStack = fuelCell.getpowerfromstack()
			data.PowerFromExternal = fuelCell.getpowerfromexternal()

			//			(AnodePressure, Power, FaultA, FaultB, FaultC, FaultD, OutletTemp, InletTemp
			//			, Volts, Amps, Cell, Fault, Run, Inactive, Standby, DCDC_Disabled, OnLoad
			//			, FanPulse, Derated, SV01, SV02, SV04, LouverOpen, DCDC_Enabled, PowerFromStack, PowerFromExternal)
			_, err = pDB.Exec(strCommand,
				data.AnodePressure, data.Power, data.FaultA, data.FaultB, data.FaultC, data.FaultD, data.OutletTemp, data.InletTemp,
				data.Volts, data.Amps, data.Cell, data.Fault, data.Run, data.Inactive, data.Standby, data.DCDCDisabled, data.OnLoad,
				data.FanPulse, data.Derated, data.SV01, data.SV02, data.SV04, data.LouverOpen, data.DCDCEnabled, data.PowerFromStack, data.PowerFromExternal)
			if err != nil {
				log.Printf("Error writing fuel cell values to the database - %s", err)
				_ = pDB.Close()
				pDB = nil
			}
		}
	}
}

/**
Get fuel cell recorded values
*/
func getFuelCellHistory(w http.ResponseWriter, r *http.Request) {
	type Row struct {
		Logged           string
		FC1Volts         float64
		FC1Current       float64
		FC1Power         float64
		FC1AnodePressure float64
		FC2Volts         float64
		FC2Current       float64
		FC2Power         float64
		FC2AnodePressure float64
		H2Pressure       float64
		GasOn            bool
		Fc1Enable        bool
		Fc1Run           bool
		Fc2Enable        bool
		Fc2Run           bool
	}

	var results []*Row
	// Set the returned type to application/json
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	from := vars["from"]
	to := vars["to"]

	var (
		tFrom time.Time
		tTo   time.Time
		rows  *sql.Rows
	)

	tFrom, err := time.Parse("2006-1-2 15:4", from)
	if err == nil {
		tTo, err = time.Parse("2006-1-2 15:4", to)
	}
	if err != nil {
		ReturnJSONError(w, "Fuel Cell", err, http.StatusBadRequest, true)
		return
	}
	if tTo.Sub(tFrom) < 0 {
		ReturnJSONErrorString(w, "Fuel Cell", "From time must be before To time.", http.StatusBadRequest, true)
		return
	}
	// If one hour or less return all data otherwise average out over one minute intervals
	if tTo.Sub(tFrom) <= time.Hour {
		rows, err = pDB.Query(`SELECT UNIX_TIMESTAMP(logged), IFNULL(fc1OutputVoltage,0), IFNULL(fc1OutputCurrent,0), IFNULL(fc1OutputPower,0), IFNULL(fc1AnodePressure, 0), 
       IFNULL(fc2OutputVoltage,0), IFNULL(fc2OutputCurrent,0), IFNULL(fc2OutputPower,0), IFNULL(fc2AnodePressure, 0), gasTankPressure
  FROM firefly.logging
  WHERE logged BETWEEN ? and ?`, from, to)

	} else {
		rows, err = pDB.Query(`SELECT (UNIX_TIMESTAMP(logged) DIV 60) * 60, IFNULL(AVG(fc1OutputVoltage),0), IFNULL(AVG(fc1OutputCurrent),0), IFNULL(AVG(fc1OutputPower),0), IFNULL(AVG(fc1AnodePressure),0),
       IFNULL(AVG(fc2OutputVoltage),0), IFNULL(AVG(fc2OutputCurrent),0), IFNULL(AVG(fc2OutputPower),0), IFNULL(AVG(fc1AnodePressure),0), AVG(gasTankPressure)
  FROM firefly.logging
  WHERE logged BETWEEN ? and ?
  GROUP BY UNIX_TIMESTAMP(logged) DIV 60`, from, to)
	}

	if err != nil {
		ReturnJSONError(w, "Fuel Cell", err, http.StatusInternalServerError, true)
		return
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Println("Error closing query - ", err)
		}
	}()
	for rows.Next() {
		row := new(Row)
		if err := rows.Scan(&(row.Logged), &(row.FC1Volts), &(row.FC1Current), &(row.FC1Power), &(row.FC1AnodePressure),
			&(row.FC2Volts), &(row.FC2Current), &(row.FC2Power), &(row.FC2AnodePressure), &(row.H2Pressure)); err != nil {
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
Turn on the fuel cell
*/
func setFcOn(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 1) || (device < 0) {
		log.Print("Invalid fuel cell in 'on' request")
		getStatus(w, r)
		return
	}
	err = turnOnFuelCell(device)
	if err != nil {
		ReturnJSONError(w, "Fuel Cell", err, http.StatusInternalServerError, true)
	} else {
		returnJSONSuccess(w)
	}
}

/**
Turn off the fuel cell
*/
func setFcOff(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 1) || (device < 0) {
		log.Print("Invalid fuel cell in 'off' request")
		getStatus(w, r)
		return
	}
	err = turnOffFuelCell(device)
	if err != nil {
		ReturnJSONError(w, "Fuel Cell", err, http.StatusInternalServerError, true)
		return
	}
	returnJSONSuccess(w)
}

/**
Start the fuel cell
*/
func setFcRun(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 1) || (device < 0) {
		ReturnJSONErrorString(w, "Fuel Cell", "Invalid fuel cell in 'run' request", http.StatusBadRequest, true)
		return
	}

	device = device
	err = startFuelCell(device)

	if err != nil {
		ReturnJSONError(w, "Fuel Cell", err, http.StatusInternalServerError, true)
		return
	}
	returnJSONSuccess(w)
}

/**
Stop the fuel cell
*/
func setFcStop(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 1) || (device < 0) {
		ReturnJSONErrorString(w, "Fuel Cell", "Invalid fuel cell in 'on' request", http.StatusInternalServerError, true)
		return
	}
	err = stopFuelCell(device)
	if err != nil {
		ReturnJSONError(w, "Fuel Cell", err, http.StatusInternalServerError, true)
		return
	}
	returnJSONSuccess(w)
}

/***
turnOnFuelCell first checks then turns on the fuel cell if it is off with a delay of 2 sconds to allow the deivce tot come on line. device is 0 based
*/
func turnOnFuelCell(device int64) error {
	switch device {
	case 0:
		if SystemStatus.Relays.FuelCell1Enable {
			return nil
		}
	case 1:
		if SystemStatus.Relays.FuelCell2Enable {
			return nil
		}
	default:
		log.Print("Cannot turn on unknown device %d", device)
		return fmt.Errorf("Unknown device %d", device)
	}
	strCommand := fmt.Sprintf("fc on %d", device)
	if len(canBus.fuelCell) > int(device) {
		canBus.fuelCell[uint16(device)].Clear()
	}
	if params.FuelCellLogOnEnable {
		canBus.setEventDateTime()
	}
	debugPrint("Turning fuel cell", device, "on")
	if _, err := sendCommand(strCommand); err != nil {
		log.Print(err)
		return err
	}

	//	log.Println("Fuel cell", device, "turned on")
	time.Sleep(time.Second * 2)
	return nil
}

/**
startFuelCell turns on then starts the fuel cell - device is 0 based
	Also starts an on demand trace
*/
func startFuelCell(device int64) error {
	if err := turnOnFuelCell(device); err != nil {
		log.Print(err)
	}
	if err := turnOnGas(); err != nil {
		log.Print(err)
	}

	strCommand := fmt.Sprintf("fc run %d", device)
	//	log.Println("Starting fuel cell ", device)
	if _, err := sendCommand(strCommand); err != nil {
		log.Print(err)
		return err
	}
	// If we are supposed to be logging all the time the fuel cell is running set the event time
	if params.FuelCellLogOnRun && !params.FuelCellLogOnEnable {
		canBus.setEventDateTime()
	}
	return nil
}

/**
stopFuelCell stops the fuel cell - device is 0 based
*/
func stopFuelCell(device int64) error {
	switch device {
	case 0:
		if !SystemStatus.Relays.FuelCell1Run {
			return nil
		}
	case 1:
		if !SystemStatus.Relays.FuelCell2Run {
			return nil
		}
	default:
		log.Print("Cannot stop unknown device %d", device)
		return fmt.Errorf("Unknown device %d", device)
	}
	// Turn the gas off first to prevent an overpressure spike when the Fuel Cell suddenly stops
	if err := turnOffGas(); err != nil {
		log.Print(err)
	}

	strCommand := fmt.Sprintf("fc stop %d", device)
	if _, err := sendCommand(strCommand); err != nil {
		log.Print(err)
		return err
	}
	if params.FuelCellLogOnRun && !params.FuelCellLogOnEnable {
		// Stop the canbus log in fifteen seconds
		canBus.setOnDemandRecording(time.Now().Add(time.Second * 15))
	}
	//	log.Println("Fuel cell", device, "Stopped")
	time.Sleep(time.Second * 10)
	return nil
}

/***
turnOffFuelCell first stops then turns off the fuel cell. device is 0 based
*/
func turnOffFuelCell(device int64) error {
	if err := stopFuelCell(device); err != nil {
		log.Print(err)
	}

	strCommand := fmt.Sprintf("fc off %d", device)
	//	log.Println("Turning fuel cell", device, "off")
	if _, err := sendCommand(strCommand); err != nil {
		log.Print(err)
		return err
	}
	//	log.Println("Fuel cell", device, "turned off")
	return nil
}

func restartFc(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 1) || (device < 0) {
		ReturnJSONErrorString(w, "Fuel Cell", "Invalid fuel cell in 'on' request", http.StatusBadRequest, true)
		return
	}
	if device == 0 {
		if err := turnOffFuelCell(0); err != nil {
			log.Print(err)
		}
		delayTime := OFFTIMEFORFUELCELLRESTART
		if len(canBus.fuelCell) > 0 {
			// If this is not the first restart, add 10 seconds delay for each time we have tried
			delayTime = OFFTIMEFORFUELCELLRESTART + (time.Second * time.Duration(canBus.fuelCell[0].NumRestarts*10))
		}
		time.AfterFunc(delayTime, func() {
			if err := startFuelCell(0); err != nil {
				log.Println("Error starting fuel cell 0 -", err)
			}
		})
	} else {
		if err := turnOffFuelCell(1); err != nil {
			log.Print(err)
		}
		time.AfterFunc(OFFTIMEFORFUELCELLRESTART, func() {
			if err := startFuelCell(1); err != nil {
				log.Println("Error starting fuel cell 1 -", err)
			}
		})
	}
	returnJSONSuccess(w)
}

func fcStatus(w http.ResponseWriter, r *http.Request) {
	var jErr JSONError
	var jStatus struct {
		On        bool    `json:"on"`
		Power     int16   `json:"power"`
		Volts     float32 `json:"volts"`
		Amps      float32 `json:"amps"`
		InletTemp float32 `json:"temp"`
	}
	vars := mux.Vars(r)
	device, err := strconv.ParseInt(vars["device"], 10, 8)
	if (err != nil) || (device > 1) || (device < 0) {
		log.Println(jErr.AddErrorString("Fuel Cell", "Invalid fuel cell in 'status' request"))
		jErr.ReturnError(w, 400)
		return
	}

	if device == 0 {
		jStatus.On = SystemStatus.Relays.FuelCell1Run
	} else {
		jStatus.On = SystemStatus.Relays.FuelCell2Run
	}
	if int(device) >= len(canBus.fuelCell) {
		if device > 0 {
			log.Println(jErr.AddErrorString("Fuel Cell", "Device invalid"))
			jErr.ReturnError(w, 404)
			return
		}
		jStatus.Amps = 0.0
		jStatus.Volts = 0.0
		jStatus.Power = 0.0
		jStatus.InletTemp = 0.0
	} else {
		jStatus.Amps = canBus.fuelCell[uint16(device)].getOutputCurrent()
		jStatus.Volts = canBus.fuelCell[uint16(device)].getOutputVolts()
		jStatus.Power = canBus.fuelCell[uint16(device)].getOutputPower()
		jStatus.InletTemp = canBus.fuelCell[uint16(device)].getInletTemp()
	}
	strResponse, err := json.Marshal(jStatus)
	if err != nil {
		ReturnJSONError(w, "Fuel Cell", err, http.StatusInternalServerError, true)
		return
	}
	_, err = fmt.Fprintf(w, string(strResponse))
	return
}

func setFcOnOff(w http.ResponseWriter, r *http.Request) {
	var jBody struct {
		OnOff bool `json:"fuelcell"`
	}
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
	if jBody.OnOff {
		setFcRun(w, r)
		debugPrint("Fuel cell turned on")
	} else {
		setFcOff(w, r)
		debugPrint("Fuel cell turned off")
	}
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
