package main

import (
	"github.com/simonvetter/modbus"
	"html"
	"log"
	"math"
	"time"
)

const ElIdle = 2
const ElStandby = 4

type electrolyserEvents struct {
	count uint16
	codes [31]uint16
}

type electrolyteLevel int

const (
	empty electrolyteLevel = iota
	low
	medium
	high
	veryHigh
)

func (l electrolyteLevel) String() string {
	switch l {
	case empty:
		return "Empty"
	case low:
		return "Low"
	case medium:
		return "Medium"
	case high:
		return "High"
	case veryHigh:
		return "Very High"
	}
	return "ERROR bad level"
}

type electrolyserStatus struct {
	Device uint8
	//	Model                 string
	//	Firmware              string
	//	Serial                string
	SystemState           uint16             // 18
	H2Flow                float32            // 1008
	ElState               uint16             // 1200
	ElectrolyteLevel      electrolyteLevel   // (7000 - 7003 four booleans)
	StackCurrent          float32            // 7508
	StackVoltage          float32            // 7510
	InnerH2Pressure       float32            // 7512
	OuterH2Pressure       float32            // 7514
	WaterPressure         float32            // 7516
	ElectrolyteTemp       float32            // 7518
	CurrentProductionRate float32            // H1002
	DefaultProductionRate float32            // H4396
	MaxTankPressure       float32            // H4308
	RestartPressure       float32            // H4310
	Warnings              electrolyserEvents // 768
	Errors                electrolyserEvents // 832
	DryerTemp1            float32
	DryerTemp2            float32
	DryerTemp3            float32
	DryerTemp4            float32
	DryerInputPressure    float32
	DryerOutputPressure   float32
	DryerErrors           uint16
	DryerWarnings         uint16
}

type Electrolyser struct {
	status             electrolyserStatus
	OnOffTime          time.Time
	ip                 string
	Client             *modbus.ModbusClient
	clientConnected    bool
	lastConnectAttempt time.Time
}

func NewElectrolyser(ip string) *Electrolyser {
	e := new(Electrolyser)
	e.OnOffTime = time.Now()
	e.ip = ip

	var config modbus.ClientConfiguration
	config.Timeout = 1 * time.Second // 5 second timeout
	config.URL = "tcp://" + ip + ":502"
	if Client, err := modbus.NewClient(&config); err != nil {
		if err != nil {
			log.Print("New modbus client error - ", err)
			return nil
		}
	} else {
		e.Client = Client
	}
	if err := e.Client.Open(); err != nil {
		log.Print("Modbus client.open error - ", err)
	} else {
		e.clientConnected = true
	}
	return e
}

func (e *Electrolyser) CheckConnected() bool {
	if e.Client == nil {
		return false
	}
	if !e.clientConnected {
		if time.Since(e.lastConnectAttempt) > time.Minute {
			if err := e.Client.Open(); err != nil {
				log.Print("Modbus client.open error - ", err)
			} else {
				e.clientConnected = true
			}
			e.lastConnectAttempt = time.Now()
		}
	}
	return e.clientConnected
}

func (e *Electrolyser) readEvents() {
	if !e.CheckConnected() {
		return
	}
	events, err := e.Client.ReadRegisters(768, 32, modbus.INPUT_REGISTER)
	if err != nil {
		log.Print("Modbus read register error - ", err)
		if err := e.Client.Close(); err != nil {
			log.Print("Error closing modbus client - ", err)
		}
		e.clientConnected = false
		return
	}
	e.status.Warnings.count = events[0]
	copy(e.status.Warnings.codes[:], events[1:])
	events, err = e.Client.ReadRegisters(832, 32, modbus.INPUT_REGISTER)
	if err != nil {
		log.Print("Modbus read register error - ", err)
		if err := e.Client.Close(); err != nil {
			log.Print("Error closing modbus client - ", err)
		}
		e.clientConnected = false
		return
	}
	e.status.Errors.count = events[0]
	copy(e.status.Errors.codes[:], events[1:])
}

func (e *Electrolyser) ReadValues() {
	if !e.CheckConnected() {
		return
	}
	values, err := e.Client.ReadFloat32s(7508, 6, modbus.INPUT_REGISTER)
	if err != nil {
		log.Print("Modbus reading float32 values - ", err)
		if err := e.Client.Close(); err != nil {
			log.Print("Error closing modbus client - ", err)
		}
		e.clientConnected = false
		return
	}
	e.status.StackCurrent = values[0]
	e.status.StackVoltage = values[1]
	e.status.InnerH2Pressure = values[2]
	e.status.OuterH2Pressure = values[3]
	e.status.WaterPressure = values[4]
	e.status.ElectrolyteTemp = values[5]

	e.status.SystemState, err = e.Client.ReadRegister(18, modbus.INPUT_REGISTER)
	if err != nil {
		log.Print("System state error - ", err)
		if err := e.Client.Close(); err != nil {
			log.Print("Error closing modbus client - ", err)
		}
		e.clientConnected = false
		return
	}

	e.status.H2Flow, err = e.Client.ReadFloat32(1008, modbus.INPUT_REGISTER)
	if err != nil {
		log.Print("H2Flow error - ", err)
		if err := e.Client.Close(); err != nil {
			log.Print("Error closing modbus client - ", err)
		}
		e.clientConnected = false
		return
	}
	// Flow will return NaN if the electrolyser is not producing.
	if math.IsNaN(float64(e.status.H2Flow)) {
		e.status.H2Flow = 0
	}

	e.status.ElState, err = e.Client.ReadRegister(1200, modbus.INPUT_REGISTER)
	if err != nil {
		log.Print("ElState - ", err)
		if err := e.Client.Close(); err != nil {
			log.Print("Error closing modbus client - ", err)
		}
		e.clientConnected = false
		return
	}

	level, err := e.Client.ReadRegisters(7000, 4, modbus.INPUT_REGISTER)
	if err != nil {
		log.Print("Electrolye Level - ", err)
		if err := e.Client.Close(); err != nil {
			log.Print("Error closing modbus client - ", err)
		}
		e.clientConnected = false
		return
	}
	switch {
	case level[2] == 0:
		e.status.ElectrolyteLevel = empty
	case level[3] == 0:
		e.status.ElectrolyteLevel = low
	case level[0] == 0:
		e.status.ElectrolyteLevel = medium
	case level[1] == 0:
		e.status.ElectrolyteLevel = high
	default:
		e.status.ElectrolyteLevel = veryHigh
	}

	e.status.CurrentProductionRate, err = e.Client.ReadFloat32(1002, modbus.HOLDING_REGISTER)
	if err != nil {
		log.Print("Current Production error - ", err)
		if err := e.Client.Close(); err != nil {
			log.Print("Error closing modbus client - ", err)
		}
		e.clientConnected = false
		return
	}

	e.readEvents()

	dryer, err := e.Client.ReadFloat32s(6002, 6, modbus.INPUT_REGISTER)
	if err != nil {
		log.Print("Error reading dryer values - ", err)
		if err := e.Client.Close(); err != nil {
			log.Print("Error closing modbus client - ", err)
		}
		e.clientConnected = false
		return
	}
	e.status.DryerTemp1 = dryer[0]
	e.status.DryerTemp2 = dryer[1]
	e.status.DryerTemp3 = dryer[2]
	e.status.DryerTemp4 = dryer[3]
	e.status.DryerInputPressure = dryer[4]
	e.status.DryerOutputPressure = dryer[5]

	dryerErrors, err := e.Client.ReadRegisters(6000, 2, modbus.INPUT_REGISTER)
	if err != nil {
		log.Print("Error reading dryer errors - ", err)
		if err := e.Client.Close(); err != nil {
			log.Print("Error closing modbus client - ", err)
		}
		e.clientConnected = false
		return
	}
	e.status.DryerErrors = dryerErrors[0]
	e.status.DryerWarnings = dryerErrors[1]
}

func (e *Electrolyser) GetSystemState() string {
	switch e.status.SystemState {
	case 0:
		return "Internal Error, System not Initialized yet"
	case 1:
		return "System in Operation"
	case 2:
		return "Error"
	case 3:
		return "System in Maintenance Mode"
	case 4:
		return "Fatal Error"
	case 5:
		return "System in Expert Mode"
	default:
		return "Unknown state"
	}
}

func decodeMessage(w uint16) string {
	switch w {
	case 0:
		return "No error"
	case 0x0FFF:
		return "Hardware failure : Unexpected error"
	case 0x1F81:
		return "Voltage < 2.9V : Brownout detected"
	case 0x1F82:
		return "Updated firmware has new mandatory settings : New parameters have been added to the configuration"
	case 0x1F83:
		return "Hardware failure : Broken periphery"
	case 0x3F84:
		return "Power Button pressed for longer than 5sec	Sticky button : Power button is pushed."
	case 0x3F85:
		return "Too low battery. : Mainbord battery charge is to low."
	case 0x108A:
		return "Pump broken. : The electrolyte pump may be damaged."
	case 0x1114:
		return "Pressure drop > 2% : Possible hydrogen leak"
	case 0x318A:
		return "Pressure > 5barg : Water inlet pressure too high"
	case 0x3194:
		return "Pressure < 1.0barg : Water inlet pressure too low	Please provide water input pressure to the water inlet."
	case 0x118A:
		return "Water level is over very high level switch : Electrolyte level is too high. Please switch the electrolyser into maintenance mode and decrease the electrolyte level."
	case 0x1194:
		return "Water level is below low level switch	Electrolyte level too low. Please switch the electrolyser into maintenance mode, drain fully and then fill the electrolyte tank with fresh electolyte solution."
	case 0x11B2:
		return "Conflict between water level sensors (low and medium level)"
	case 0x11B3:
		return "Conflict between water level sensors (medium and high level)"
	case 0x11B4:
		return "Conflict between water level sensors (high and very high level)"
	case 0x11A8:
		return "Refilling unsuccessful."
	case 0x3195:
		return "Refilling timeout	Please reboot device and ensure water inlet requirements are met."
	case 0x3196:
		return "Refilling failure	The refilling faled. Check water water supply system."
	case 0x31B3:
		return "Available only in Maintenance Mode	Drain completely	Electrolyte level is below minimum level. Electrolyser is ready for refill."
	case 0x31B4:
		return "Available only in Maintenance Mode	Refill to high level	Please continue filling the electrolyte."
	case 0x31B5:
		return "Electrolyte level is very high, drain to high level."
	case 0x1201:
		return "PSU bad current. PSU might be broken."
	case 0x120A:
		return "Broken membrane. Memrrane inside the stack might be broken."
	case 0x3215:
		return "Pressure spike > 2%	Drifting PT101A. Pressure mismatch towards stack status has been detected."
	case 0x3216:
		return "System works with electrolyte level less than medium one and can not refill (during pressure limit and etc)	Refilling not happening	Please check the water supply - otherwise, the hydrogen production will stop soon."
	case 0x321E:
		return "Stack voltage is too high	Replace electrolyte	Replace electrolyte. If the errror persists."
	case 0x128A:
		return "Temperature > 58°C	Electrolyte temperature too high	Please make sure that air ventilation is unobstructed or cooling liquid cooling loop operating and that ambient temperatures do not exceed device specifiations"
	case 0x3294:
		return "Rotation < 600rpm	Electrolyte cooling fan broken. The electrolyte cooling fan should be checked."
	case 0x228A:
		return "Temperature < 6°C	Electrolyte temperature too low	Please make sure that room temperature is at least 6°C. Keep the EL powered to ensure the heating routine continues to protect the device internals."
	case 0x330A:
		return "Pressure is > atmospheric pressure + 10%	Gas side pressure is not atmospheric	Purge line pressure detected. Ramp-Up is not possible. Please check that the purge line is unobstructed."
	case 0x230A:
		return "Cannot start the heater because the water level in the internal electrolyser tank is too low.	Not enough warmup water	Heater can't be started due to a low electrolyte level. Refill electrolyser, restart and try again."
	case 0x1401:
		return "Pressure > 37bar	Hydrogen inner pressure too high. The hydrogen inner pressure exceeded 37 barg (nominal, but high)."
	case 0x1402:
		return "Water sensor is wet	Water presence. Water is leaking inside the electrolyser. Please remove the water supply and power from the system and drain immediately."
	case 0x1403:
		return "No voltage from PSU	PSU broken. PSU failure detected. No voltage on stack."
	case 0x1404:
		return "Current > 58A	Stack current too high. Stack overcurrent detected."
	case 0x1405:
		return "Backflow temperature too high. The stack outlet temperature is too high."
	case 0x1407:
		return "Temperature > 75°C	Electronic board temperature too high	The electronic board temperature is too high. Please check and clean ventialtion openings."
	case 0x1408:
		return "vent line obstruction	Electrolyte tank pressure too high	Please make sure that O2 vent line is not blocked."
	case 0x1409:
		return "Electrolyte temperature too low	Please make sure that room temperature is at least 6°C. Keep the EL powered to ensure the heating routine continues to protect the device internals."
	case 0x140A:
		return "Hydrogen pressure too high. pressure transmitter calibration needs to be verified."
	case 0x140B:
		return "Temperature Sensor	Temperature > 75°C	Control Board MCU temperature too high	Please make sure that room temperature below 45°C."
	case 0x141E:
		return "Water inlet pressure transmitter broken. The water inlet pressure cannot be measured or bad water inlet pressure."
	case 0x141F:
		return "Electrolyte tank temperature transmitter broken. The electrolyte tank temperature cannot be measured."
	case 0x1420:
		return "Electrolyte flow meter broken. The electrolyte flow cannot be measured."
	case 0x1421:
		return "Electrolyte backflow temperature transmitter broken. The electrolyte backflow temperature cannot be measured."
	case 0x1422:
		return "Hydrogen inner pressure transmitter broken. The hydrogen inner pressure cannot be measured."
	case 0x1423:
		return "Outer hydrogen pressure transmitter broken. The outer hydrogen pressure cannot be measured."
	case 0x1424:
		return "Rotation < 3000rpm	Chassis circulation fan broken. The chassis air circulation fan speed cannot be measured."
	case 0x1425:
		return "Rotation < 3000rpm	Electronic compartment cooling fan broken. The electronic compartment cooling fan speed cannot be measured."
	case 0x1426:
		return "Electronic board temperature transmitter broken. The electronic board temperature cannot be measured."
	case 0x1427:
		return "Current sensor broken. The stack current cannot be measured."
	case 0x1428:
		return "Dry contact error	Dry contact triggered system stop. Please check your system to understand what triggered the dry contact."
	case 0x3432:
		return "Hydrogen inner pressure check disabled."
	case 0x3433:
		return "Water presence check disabled."
	case 0x3434:
		return "PSU check disabled."
	case 0x3435:
		return "Stack current check disabled."
	case 0x3436:
		return "Backflow temperature check disabled."
	case 0x3437:
		return "Electronic board temperature check disabled"
	case 0x3438:
		return "Electrolyte tank pressure check disabled."
	case 0x3439:
		return "Low electrolyte temperature check disabled."
	case 0x343B:
		return "Inlet pressure check disabled."
	case 0x343C:
		return "Electrolyte tank temperature check disabled."
	case 0x343D:
		return "Electrolyte flow meter check disabled."
	case 0x343E:
		return "Electrolyte cooling check disabled."
	case 0x343F:
		return "Electrolyte backflow temperature check disabled."
	case 0x3440:
		return "Hydrogen outer pressure check disabled."
	case 0x3441:
		return "Chassis circulation fan check disabled."
	case 0x3442:
		return "Electronic compartment cooling fan check disabled."
	case 0x3443:
		return "External switch		Dry contact check disabled."
	case 0x3445:
		return "MCU Temperature Sensor		Control Board MCU temperature check disabled."
	case 0x148A:
		return "Frozen pipes. Electrolyte flow outside pump control limits."
	case 0x1501:
		return "Possible hydrogen leak detected. Pressure readings below nominal values. The device needs to be checked or repaired."
	case 0x350A:
		return "Insufficient pressure drop	Insufficient pressure drop. Check that purge line from the electrolyser is not obstructed."
	case 0x358A:
		return "Pressure > 25 barg	Outer pressure is too high to run blowdown routine	Please reduce outlet pressure to below 25 bar in order to run the blowdown routine."
	case 0x3594:
		return "The Blowdown procedure will be started at H2 production start	Blowdown Routine Active. Please make sure that purge line is properly connected and leads to a safe area."
	case 0x159E:
		return "The purge line is obstructed"
	case 0x360A:
		return "ModBus	Heartbeat Packet was not received in time : Lost ModBus safety heartbeat communication : Please check ModBus communication between Electrolyser and controller. Please check if Ethernet cable is properly installed and connection is established."
	case 0x360B:
		return "Gateway	Heartbeat Packet was not received in time : Lost Gateway safety heartbeat communication : Please check communication between Gateway and Electrolyser (UCM). Please check if WiFi connection is stable."
	case 0x360C:
		return "Heartbeat Packet was not received in time : Lost UCM safety heartbeat communication"
	case 0x368A:
		return "Polarization curve cannot be started."
	default:
		return "Unknown Error/Warning"
	}
}

func (e *Electrolyser) GetWarnings() []string {
	var s []string

	for w := uint16(0); w < e.status.Warnings.count; w++ {
		s = append(s, decodeMessage(e.status.Warnings.codes[w]))
	}
	return s
}

func (e *Electrolyser) GetErrors() []string {
	var s []string

	for err := uint16(0); err < e.status.Errors.count; err++ {
		s = append(s, decodeMessage(e.status.Errors.codes[err]))
	}
	return s
}

func (e *Electrolyser) getState() string {
	switch e.status.ElState {
	case 0:
		return "Halted"
	case 1:
		return "Maintenance mode"
	case 2:
		return "Idle"
	case 3:
		return "Steady"
	case 4:
		return "Stand-By"
	case 5:
		return "Curve"
	case 6:
		return "Blowdown"
	default:
		return "Unknown State"
	}
}

func decodeDryerMessage(code uint16) []string {
	var e []string
	for b := uint16(1); b < 0b1000000000000000; b <<= 1 {
		if (code & b) > 0 {
			switch b {
			case 0b1:
				e = append(e, "TT00 has invalid value (sensor provides unexpected values)")
			case 0b10:
				e = append(e, "TT01 has invalid value (sensor provides unexpected values)")
			case 0b100:
				e = append(e, "TT02 has invalid value (sensor provides unexpected values)")
			case 0b1000:
				e = append(e, "TT03 has invalid value (sensor provides unexpected values)")
			case 0b10000:
				e = append(e, "TT00 value growth is not enough (heating mechanism does not work properly)")
			case 0b100000:
				e = append(e, "TT01 value growth is not enough (heating mechanism does not work properly)")
			case 0b1000000:
				e = append(e, "TT02 value growth is not enough (heating mechanism does not work properly)")
			case 0b10000000:
				e = append(e, "TT03 value growth is not enough (heating mechanism does not work properly)")
			case 0b100000000:
				e = append(e, "PS00 (pressure switch on line 0) is triggered")
			case 0b1000000000:
				e = append(e, "PS01 (pressure switch on line 1) is triggered")
			case 0b10000000000:
				e = append(e, "F100 has invalid RPM speed (fan between line 0 and line 1)")
			case 0b100000000000:
				e = append(e, "F101 has invalid RPM speed (fan on line 0)")
			case 0b1000000000000:
				e = append(e, "F102 has invalid RPM speed (fan on line 1)")
			case 0b10000000000000:
				e = append(e, "PT00 (Input pressure) has invalid value (sensor provides unexpected values)")
			case 0b100000000000000:
				e = append(e, "PT01 (Output pressure) has invalid value (sensor provides unexpected values)")
			}
		}
	}
	return e
}

func (e *Electrolyser) GetDryerErrors() []string {
	return decodeDryerMessage(e.status.DryerErrors)
}

func (e *Electrolyser) GetDryerWarnings() []string {
	return decodeDryerMessage(e.status.DryerWarnings)
}

func (e *Electrolyser) SetProduction(rate uint8) {
	if !e.CheckConnected() {
		return
	}
	if rate < 60 {
		e.Stop()
	} else {
		err := e.Client.WriteFloat32(1002, float32(rate))
		if err != nil {
			log.Print("Error setting production rate - ", err)
			if err := e.Client.Close(); err != nil {
				log.Print("Error closing modbus client - ", err)
			}
			e.clientConnected = false
		}
	}
}

func (e *Electrolyser) Start() {
	if !e.CheckConnected() {
		return
	}
	err := e.Client.WriteRegister(1000, 1)
	if err != nil {
		log.Print("Error starting Electrolyser - ", err)
		if err := e.Client.Close(); err != nil {
			log.Print("Error closing modbus client - ", err)
		}
		e.clientConnected = false
	}
}

func (e *Electrolyser) Stop() {
	if !e.CheckConnected() {
		return
	}
	err := e.Client.WriteRegister(1000, 0)
	if err != nil {
		log.Print("Error stopping electrolyser - ", err)
		if err := e.Client.Close(); err != nil {
			log.Print("Error closing modbus client - ", err)
		}
		e.clientConnected = false
	}
}

func (e *Electrolyser) Preheat() {
	if !e.CheckConnected() {
		return
	}
	err := e.Client.WriteRegister(1014, 1)
	if err != nil {
		log.Print("Preheat Request failed - ", err)
		if err := e.Client.Close(); err != nil {
			log.Print("Error closing modbus client - ", err)
		}
		e.clientConnected = false
	}
}

func (e *Electrolyser) Reboot() {
	if !e.CheckConnected() {
		return
	}
	err := e.Client.WriteRegister(4, 1)
	if err != nil {
		log.Print("Reboot Request failed - ", err)
		if err := e.Client.Close(); err != nil {
			log.Print("Error closing modbus client - ", err)
		}
		e.clientConnected = false
	}
}

func (e *Electrolyser) Locate() {
	if !e.CheckConnected() {
		return
	}
	err := e.Client.WriteRegister(5, 1)
	if err != nil {
		log.Print("Locate Request failed - ", err)
		if err := e.Client.Close(); err != nil {
			log.Print("Error closing modbus client - ", err)
		}
		e.clientConnected = false
	}
}

func (e *Electrolyser) EnableMaintenance() {
	if !e.CheckConnected() {
		return
	}
	err := e.Client.WriteRegister(6, 1)
	if err != nil {
		log.Print("Enable Maintenance Request failed - ", err)
		if err := e.Client.Close(); err != nil {
			log.Print("Error closing modbus client - ", err)
		}
		e.clientConnected = false
	}
}

func (e *Electrolyser) DisableMaintenance() {
	if !e.CheckConnected() {
		return
	}
	err := e.Client.WriteRegister(6, 0)
	if err != nil {
		log.Print("Disable Maintenance Request failed - ", err)
		if err := e.Client.Close(); err != nil {
			log.Print("Error closing modbus client - ", err)
		}
		e.clientConnected = false
	}
}

func (e *Electrolyser) Blowdown() {
	if !e.CheckConnected() {
		return
	}
	err := e.Client.WriteRegister(1010, 1)
	if err != nil {
		log.Print("Blowdown Request failed - ", err)
		if err := e.Client.Close(); err != nil {
			log.Print("Error closing modbus client - ", err)
		}
		e.clientConnected = false
	}
}

func (e *Electrolyser) Refill() {
	if !e.CheckConnected() {
		return
	}
	err := e.Client.WriteRegister(1011, 1)
	if err != nil {
		log.Print("Refil Request failed - ", err)
		if err := e.Client.Close(); err != nil {
			log.Print("Error closing modbus client - ", err)
		}
		e.clientConnected = false
	}
}

func (e *Electrolyser) StartDryer() {
	if !e.CheckConnected() {
		return
	}
	err := e.Client.WriteRegister(6018, 1)
	if err != nil {
		log.Print("Start Dryer Request failed - ", err)
		if err := e.Client.Close(); err != nil {
			log.Print("Error closing modbus client - ", err)
		}
		e.clientConnected = false
	}
}

func (e *Electrolyser) StopDryer() {
	if !e.CheckConnected() {
		return
	}
	err := e.Client.WriteRegister(6019, 1)
	if err != nil {
		log.Print("Stop Dryer Request failed - ", err)
		if err := e.Client.Close(); err != nil {
			log.Print("Error closing modbus client - ", err)
		}
		e.clientConnected = false
	}
}

func (e *Electrolyser) RebootDryer() {
	if !e.CheckConnected() {
		return
	}
	err := e.Client.WriteRegister(6020, 1)
	if err != nil {
		log.Print("Reboot Dryer Request failed - ", err)
		if err := e.Client.Close(); err != nil {
			log.Print("Error closing modbus client - ", err)
		}
		e.clientConnected = false
	}
}

func (e *Electrolyser) GetDryerErrorsHTML() string {
	var htmlString = "<table>"

	for _, err := range e.GetDryerErrors() {
		htmlString += "<tr><td>" + html.EscapeString(err) + "</td></tr>"
	}
	htmlString += "</table>"
	return htmlString
}

func (e *Electrolyser) GetDryerErrorText() string {
	var s = ""

	for _, err := range e.GetDryerErrors() {
		if s != "" {
			s += "\n"
		}
		s += err
	}
	return s
}

func (e *Electrolyser) GetDryerWarningsHTML() string {
	var htmlString = "<table>"

	for _, warning := range e.GetDryerWarnings() {
		htmlString += "<tr><td>" + html.EscapeString(warning) + "</td></tr>"
	}
	htmlString += "</table>"
	return htmlString
}

func (e *Electrolyser) GetDryerWarningText() string {
	var s = ""

	for _, warning := range e.GetDryerWarnings() {
		if s != "" {
			s += "\n"
		}
		s += warning
	}
	return s
}
