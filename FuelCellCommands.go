package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
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

// Parse a string as a uint8 value
func parseDevice(device string) (uint8, error) {
	d, err := strconv.ParseUint(device, 10, 8)
	return uint8(d), err
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
		Cell           uint8
		AnodePressure  uint16
		FaultA         uint32
		FaultB         uint32
		FaultC         uint32
		FaultD         uint32
		InletTemp      int16
		OutletTemp     int16
		Power          int16
		Amps           int16
		Volts          uint16
		State          sql.NullString
		fanDutyCycle   uint16
		louverPosition uint16
		flags          uint16
	}
	strCommand := `INSERT INTO firefly.FuelCell
(Cell, AnodePressure, Power, FaultA, FaultB, FaultC, FaultD, OutletTemp, InletTemp, Volts, Amps, State, Flags, FanDutyCycle, LouverPosition)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`

	var err error
	if pDB == nil {
		if pDB, err = connectToDatabase(); err != nil {
			log.Print(err)
			return
		}
	}

	for device, fuelCell := range canBus.fuelCell {
		if fuelCell.IsSwitchedOn() {
			data.Cell = device
			data.AnodePressure = fuelCell.getAnodePressureRaw()
			data.FaultA = fuelCell.getFaultA()
			data.FaultB = fuelCell.getFaultB()
			data.FaultC = fuelCell.getFaultC()
			data.FaultD = fuelCell.getFaultD()
			data.InletTemp = fuelCell.getInletTempRaw()
			data.OutletTemp = fuelCell.getOutletTempRaw()
			data.Amps = fuelCell.getOutputCurrentRaw()
			data.Volts = fuelCell.getOutputVoltsRaw()
			data.Power = fuelCell.getOutputPower()
			if len(fuelCell.GetState()) > 0 {
				data.State.String = fuelCell.GetState()
				data.State.Valid = true
			}
			data.fanDutyCycle = fuelCell.getFanSPdutyRaw()
			data.louverPosition = fuelCell.getLouverPositionRaw()
			// Encode the flags to a bit field to save database space
			data.flags = 0
			if fuelCell.getFault() {
				data.flags |= 1
			}
			if fuelCell.getRun() {
				data.flags |= (1 << 1)
			}
			if fuelCell.getInactive() {
				data.flags |= (1 << 2)
			}
			if fuelCell.getStandby() {
				data.flags |= (1 << 3)
			}
			if fuelCell.getDCDCDisabled() {
				data.flags |= (1 << 4)
			}
			if fuelCell.getOnLoad() {
				data.flags |= (1 << 5)
			}
			if fuelCell.getFanPulse() {
				data.flags |= (1 << 6)
			}
			if fuelCell.getDerated() {
				data.flags |= (1 << 7)
			}
			if fuelCell.getSV01() {
				data.flags |= (1 << 8)
			}
			if fuelCell.getSV02() {
				data.flags |= (1 << 9)
			}
			if fuelCell.getSV04() {
				data.flags |= (1 << 10)
			}
			if fuelCell.getLouverOpen() {
				data.flags |= (1 << 11)
			}
			if fuelCell.getpowerfromstack() {
				data.flags |= (1 << 12)
			}
			if fuelCell.getpowerfromexternal() {
				data.flags |= (1 << 13)
			}
			if fuelCell.getDCDCEnabled() {
				data.flags |= (1 << 14)
			}
			//Cell, AnodePressure, Power, FaultA, FaultB, FaultC, FaultD, OutletTemp, InletTemp, Volts, Amps, State, Flags, FanDutyCycle, LouverPosition
			_, err = pDB.Exec(strCommand,
				data.Cell, data.AnodePressure, data.Power, data.FaultA, data.FaultB, data.FaultC, data.FaultD, data.OutletTemp, data.InletTemp,
				data.Volts, data.Amps, data.State, data.flags, data.fanDutyCycle, data.louverPosition)
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
		Logged            string  `json:"logged"`
		AnodePressure     float64 `json:"anodePressure"`
		Power             float64 `json:"power"`
		FaultA            string  `json:"faultA"`
		FaultB            string  `json:"faultB"`
		FaultC            string  `json:"faultC"`
		FaultD            string  `json:"faultD"`
		Volts             float64 `json:"volts"`
		Current           float64 `json:"current"`
		InletTemp         float64 `json:"inletTemp"`
		OutletTemp        float64 `json:"outletTemp"`
		Fan               float64 `json:"fan"`
		Louver            float64 `json:"louver"`
		State             string  `json:"state"`
		Fault             bool    `json:"fault"`
		Run               bool    `json:"run"`
		Inactive          bool    `json:"inactive"`
		Standby           bool    `json:"standby"`
		DCDC_Disabled     bool    `json:"dcdcDisabled"`
		OnLoad            bool    `json:"onLoad"`
		FanPulse          bool    `json:"fanPulse"`
		Derated           bool    `json:"derated"`
		SV01              bool    `json:"sv01"`
		SV02              bool    `json:"sv02"`
		SV04              bool    `json:"sv04"`
		LouverOpen        bool    `json:"louverOpen"`
		PowerFromStack    bool    `json:"powerFromStack"`
		PowerFromExternal bool    `json:"powerFromExternal"`
		DCDC_Enabled      bool    `json:"dcdcEnabled"`
	}

	var results []*Row
	// Set the returned type to application/json
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	from := vars["from"]
	to := vars["to"]
	device := vars["device"]

	var (
		tFrom  time.Time
		tTo    time.Time
		Device uint8
		rows   *sql.Rows
	)

	if tDevice, err := strconv.ParseUint(device, 10, 8); err != nil {
		ReturnJSONError(w, "FuelCell", err, http.StatusBadRequest, false)
		return
	} else {
		Device = uint8(tDevice)
	}

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
		rows, err = pDB.Query(`SELECT UNIX_TIMESTAMP(logged)
										, AnodePressure
										, Power
										, FaultA
										, FaultB
										, FaultC
										, FaultD
										, OutletTemp
										, InletTemp
										, Volts
										, Amps
										, IFNULL(State, "")
										, FanDutyCycle
										, LouverPosition
										, Fault
										, Run
										, Inactive
										, Standby
										, DCDC_Disabled
										, OnLoad
										, FanPulse
										, Derated
										, SV01
										, SV02
										, SV04
										, LouverOpen
										, PowerFromStack
										, PowerFromExternal
										, DCDC_Enabled
  FROM firefly.FuelCellData
  WHERE logged BETWEEN ? AND ? AND cell = ?`, from, to, Device)
	} else {
		rows, err = pDB.Query(`SELECT (UNIX_TIMESTAMP(logged) DIV 60) * 60
										, ROUND(AVG(AnodePressure), 1)
										, ROUND(AVG(Power), 1)
										, LAST_VALUE(FaultA)
										, LAST_VALUE(FaultB)
										, LAST_VALUE(FaultC)
										, LAST_VALUE(FaultD)
										, ROUND(AVG(OutletTemp), 1)
										, ROUND(AVG(InletTemp), 1)
										, ROUND(AVG(Volts), 1)
										, ROUND(AVG(Amps), 1)
										, IFNULL(LAST_VALUE(State), "")
										, ROUND(AVG(FanDutyCycle), 1)
										, ROUND(AVG(LouverPosition), 1)
										, LAST_VALUE(Fault)
										, LAST_VALUE(Run)
										, LAST_VALUE(Inactive)
										, LAST_VALUE(Standby)
										, LAST_VALUE(DCDC_Disabled)
										, LAST_VALUE(OnLoad)
										, LAST_VALUE(FanPulse)
										, LAST_VALUE(Derated)
										, LAST_VALUE(SV01)
										, LAST_VALUE(SV02)
										, LAST_VALUE(SV04)
										, LAST_VALUE(LouverOpen)
										, LAST_VALUE(PowerFromStack)
										, LAST_VALUE(PowerFromExternal)
										, LAST_VALUE(DCDC_Enabled)
  FROM firefly.FuelCellData
  WHERE logged BETWEEN ? AND ? AND cell = ?
  GROUP BY UNIX_TIMESTAMP(logged) DIV 60`, from, to, Device)
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
		if err := rows.Scan(&(row.Logged), &(row.AnodePressure), &(row.Power),
			&(row.FaultA), &(row.FaultB), &(row.FaultC), &(row.FaultD),
			&(row.OutletTemp), &(row.InletTemp), &(row.Volts), &(row.Current),
			&(row.State), &(row.Fan), &(row.Louver),
			&(row.Fault), &(row.Run), &(row.Inactive), &(row.Standby), &(row.DCDC_Disabled), &(row.OnLoad),
			&(row.FanPulse), &(row.Derated), &(row.SV01), &(row.SV02), &(row.SV04),
			&(row.LouverOpen), &(row.PowerFromStack), &(row.PowerFromExternal), &(row.DCDC_Enabled)); err != nil {
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

/***
turnOnFuelCell first checks then turns on the fuel cell if it is off with a delay of 2 sconds to allow the deivce tot come on line. device is 0 based
*/
func turnOnFuelCell(device uint8) error {
	switch device {
	case 0:
		if SystemStatus.Relays.FC0Enable {
			return nil
		} else {
			if len(canBus.fuelCell) > 0 {
				canBus.fuelCell[device].Clear()
			}
			if params.FuelCellLogOnEnable {
				canBus.setEventDateTime()
			}
			if err := mbusRTU.FC0OnOff(true); err != nil {
				return err
			}
		}
	case 1:
		if SystemStatus.Relays.FC1Enable {
			return nil
		} else {
			if len(canBus.fuelCell) > 1 {
				canBus.fuelCell[device].Clear()
			}
			if params.FuelCellLogOnEnable {
				canBus.setEventDateTime()
			}
			if err := mbusRTU.FC1OnOff(true); err != nil {
				return err
			}
		}
	default:
		log.Print("Cannot turn on unknown device %d", device)
		return fmt.Errorf("Unknown device %d", device)
	}
	time.Sleep(time.Second * 2)
	return nil
}

/**
startFuelCell turns on then starts the fuel cell - device is 0 based
	Also starts an on demand trace
*/
func startFuelCell(device uint8) error {
	if err := turnOnFuelCell(device); err != nil {
		log.Print(err)
	}
	if err := mbusRTU.GasOnOff(true); err != nil {
		log.Print(err)
	}

	if err := mbusRTU.FCRunStop(device, true); err != nil {
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
func stopFuelCell(device uint8) error {
	switch device {
	case 0:
		if !SystemStatus.Relays.FC0Run {
			return nil
		}
		if err := mbusRTU.FC0RunStop(false); err != nil {
			log.Print(err)
			return err
		}
	case 1:
		if !SystemStatus.Relays.FC1Run {
			return nil
		}
		if err := mbusRTU.FC1RunStop(false); err != nil {
			log.Print(err)
			return err
		}
	default:
		log.Print("Cannot stop unknown device %d", device)
		return fmt.Errorf("Unknown device %d", device)
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
PowerDown waits for the fuel cell to stop delivering power then turns the enable relay off
If it is outputting power it will wait 2 seconds and try again. After 2 minutes it will turn the fuel cell off even if it didn't stop
*/
func PowerDown(device uint8) {
	for i := 0; i < 60; i++ {
		// If the fuel cell is registered and we have data from it...
		if len(canBus.fuelCell) > int(device) {
			// If the fuel cell is not outputting power turn it off
			if canBus.fuelCell[device].OutputPower <= 0 {
				switch device {
				case 0:
					if err := mbusRTU.FC0OnOff(false); err != nil {
						log.Print(err)
					} else {
						if !mbusRTU.fc1en {
							canBus.clearEventDateTime()
							if err := mbusRTU.GasOnOff(false); err != nil {
								log.Println(err)
							}
						}
						return
					}
				case 1:
					if err := mbusRTU.FC0OnOff(false); err != nil {
						log.Print(err)
					} else {
						if !mbusRTU.fc0en {
							canBus.clearEventDateTime()
							if err := mbusRTU.GasOnOff(false); err != nil {
								log.Println(err)
							}
						}
						return
					}
				}
			} else {
				// Just to be sure, tell it to stop again
				if err := stopFuelCell(device); err != nil {
					log.Print(err)
				}
				// Wait 2 seconds and try again
				time.Sleep(time.Second * 2)
			}
		}
	}
	log.Printf("Times out waiting for fuel cell %d to stop. Turning fuel cell off now!", device)
	if err := mbusRTU.FCOnOff(device, false); err != nil {
		log.Print(err)
	}
}

/***
turnOffFuelCell first stops then turns off the fuel cell. device is 0 based
*/
func turnOffFuelCell(device uint8) error {
	if err := stopFuelCell(device); err != nil {
		log.Print(err)
	}

	go PowerDown(device)

	//	strCommand := fmt.Sprintf("fc off %d", device)
	//	log.Println("Turning fuel cell", device, "off")
	//	if _, err := sendCommand(strCommand); err != nil {
	//		log.Print(err)
	//		return err
	//	}
	//	log.Println("Fuel cell", device, "turned off")
	return nil
}

//func restartFc(w http.ResponseWriter, r *http.Request) {
//	vars := mux.Vars(r)
//	device, err := strconv.ParseInt(vars["device"], 10, 8)
//	if (err != nil) || (device > 1) || (device < 0) {
//		ReturnJSONErrorString(w, "Fuel Cell", "Invalid fuel cell in 'on' request", http.StatusBadRequest, true)
//		return
//	}
//}

func fcEnabled(device uint8) bool {
	switch device {
	case 0:
		return SystemStatus.Relays.FC0Enable
	case 1:
		return SystemStatus.Relays.FC1Enable
	default:
		log.Print("Invalid fuel cell in fcEnabled")
	}
	return false
}

func NewStartFuelCellFFunc(device uint8) func() {
	return func() {
		if err := startFuelCell(device); err != nil {
			log.Println("Error starting fuel cell %d -", device, err)
			return
		}
	}
}

func restartFc(device uint8) error {
	var pFC *FCM804
	switch device {
	case 0:
		if len(canBus.fuelCell) > 0 {
			pFC = canBus.fuelCell[0]
		}
	case 1:
		if len(canBus.fuelCell) > 1 {
			pFC = canBus.fuelCell[1]
		}
	default:
		err := fmt.Errorf("Invalid fuel cell in restart command")
		log.Print(err)
		return err
	}

	// Turn the fuel cell off first
	if err := turnOffFuelCell(0); err != nil {
		log.Print(err)
	}

	// Wait up to 3 minutes for the fuel cell to be turned off
	for i := 0; i < 180; i++ {
		if fcEnabled(device) {
			// Wait another 2 seconds and check again
			time.Sleep(time.Second * 2)
		}
	}
	if fcEnabled(device) {
		err := fmt.Errorf("Failed to turn Fuel Cell %d off.", device)
		return err
	}

	delayTime := OFFTIMEFORFUELCELLRESTART
	if pFC != nil {
		// If this is not the first restart, add 10 seconds delay for each time we have tried
		delayTime = OFFTIMEFORFUELCELLRESTART + (time.Second * time.Duration(canBus.fuelCell[0].NumRestarts*10))
	}
	time.AfterFunc(delayTime, NewStartFuelCellFFunc(device))

	return nil
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
	device, err := parseDevice(vars["device"])
	if (err != nil) || (device > 1) || (device < 0) {
		log.Println(jErr.AddErrorString("Fuel Cell", "Invalid fuel cell in 'status' request"))
		jErr.ReturnError(w, 400)
		return
	}

	if device == 0 {
		jStatus.On = SystemStatus.Relays.FC0Run
	} else {
		jStatus.On = SystemStatus.Relays.FC1Run
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
		jStatus.Amps = canBus.fuelCell[device].getOutputCurrent()
		jStatus.Volts = canBus.fuelCell[device].getOutputVolts()
		jStatus.Power = canBus.fuelCell[device].getOutputPower()
		jStatus.InletTemp = canBus.fuelCell[device].getInletTemp()
	}
	strResponse, err := json.Marshal(jStatus)
	if err != nil {
		ReturnJSONError(w, "Fuel Cell", err, http.StatusInternalServerError, true)
		return
	}
	_, err = fmt.Fprintf(w, string(strResponse))
	return
}

//func setFcOnOff(w http.ResponseWriter, r *http.Request) {
//	var jBody OnOffPayload
//
//	jBody.Device = 0xff
//
//	body, err := io.ReadAll(r.Body)
//	if err != nil {
//		ReturnJSONError(w, "Fuel Cell", err, http.StatusBadRequest, true)
//		return
//	}
//	err = json.Unmarshal(body, &jBody)
//	if err != nil {
//		ReturnJSONError(w, "Fuel Cell", err, http.StatusBadRequest, true)
//		return
//	}
//	if jBody.Device == 0xff {
//		ReturnJSONErrorString(w, "Fuel Cell", "Device was not specified in the payload", http.StatusBadRequest, true)
//		return
//	}
//
//	if jBody.State {
//		setFcRun(w, r)
//		debugPrint("Fuel cell turned on")
//	} else {
//		setFcOff(w, r)
//		debugPrint("Fuel cell turned off")
//	}
//}
