package main

import (
	"fmt"
	"github.com/simonvetter/modbus"
	"log"
	"sync"
	"time"
)

const MODBUSRTUPORT = "/dev/ttyUSB0"
const RELAYFC0EN = 8
const RELAYFC0RUN = 7
const RELAYFC1EN = 6
const RELAYFC1RUN = 5
const RELAYSPARE = 4
const RELAYEL0 = 2
const RELAYEL1 = 3
const RELAYGAS = 1
const TANKPRESSURE = 1
const FUELCELLPRESSURE = 5
const CONDUCTIVITY = 6

type ModbusRTUIO struct {
	Active            bool
	mbus              *modbus.ModbusClient
	commsPort         string
	baudRate          uint
	dataBits          uint
	stopBits          uint
	parity            string
	timeoutSecs       uint
	relaySlaveAddress uint8
	acSlaveAddress    uint8
	hpSlaveAddress    uint8
	fc0en             bool
	fc0run            bool
	fc1en             bool
	fc1run            bool
	el0               bool
	el1               bool
	gas               bool
	spare             bool

	rawConductivity     uint16
	conductivity        float32
	tankPressure        float64
	rawTankPressure     uint16
	fuelCellPressure    float32
	rawFuelCellPressure uint16
	lastIOUpdate        time.Time

	acPower       float32
	acVolts       float32
	acCurrent     float32
	acPowerFactor uint16
	acEnergy      float32
	acFrequency   float32
	lastACUpdate  time.Time

	hpPower       float32
	hpVolts       float32
	hpCurrent     float32
	hpPowerFactor uint16
	hpEnergy      float32
	hpFrequency   float32
	lastHPUpdate  time.Time

	muModbus sync.Mutex
	muBuffer sync.Mutex
}

const VOLTAGEREGISTER = 0x0000
const VOLTAGELENGTH = 1
const CURRENTREGISTER = 0x0001
const CURRENTLENGTH = 2
const POWERREGISTER = 0x0003
const POWERLENGTH = 2
const ENERGYREGISTER = 0x0005
const ENERGYLENGTH = 2
const FREQUENCYREGISTER = 0x0007
const FREQUENCYLENGTH = 1
const POWERFACTORREGISTER = 0x0008
const POWERFACTORLENGTH = 1

func NewModbusRTUIO(CommsPort string, BaudRate uint, DataBits uint, StopBits uint, Parity string, TimeoutSecs uint, RelaySlaveAddress uint8, ACSlaveAddress uint8, HPSlaveAddress uint8) *ModbusRTUIO {
	rtu := new(ModbusRTUIO)
	rtu.commsPort = CommsPort
	rtu.baudRate = BaudRate
	rtu.dataBits = DataBits
	rtu.stopBits = StopBits
	rtu.parity = Parity
	rtu.timeoutSecs = TimeoutSecs
	rtu.relaySlaveAddress = RelaySlaveAddress
	rtu.acSlaveAddress = ACSlaveAddress
	rtu.hpSlaveAddress = HPSlaveAddress
	return rtu
}

func (rtu *ModbusRTUIO) StartModbusIO() {
	modbusTicker := time.NewTicker(time.Second)
	var config modbus.ClientConfiguration

	config.Timeout = time.Second * time.Duration(rtu.timeoutSecs)
	config.DataBits = rtu.dataBits
	switch rtu.parity {
	case "N":
		config.Parity = modbus.PARITY_NONE
	case "E":
		config.Parity = modbus.PARITY_EVEN
	case "O":
		config.Parity = modbus.PARITY_ODD
	default:
		log.Println("Invalid parity for Modbus RTU provided -", rtu.parity, " setting to None")
		config.Parity = modbus.PARITY_NONE
	}
	config.StopBits = rtu.stopBits
	config.Speed = rtu.baudRate
	config.URL = rtu.commsPort

	mbus, err := modbus.NewClient(&config)
	if err != nil {
		log.Println("Modbus configration error -", err)
		return
	} else {
		rtu.mbus = mbus
	}

	err = mbus.Open()
	if err != nil {
		log.Println("Modbus open error -", err)
		return
	} else {
		log.Println("Modbus RTU is now open")
	}

	rtu.Active = true

	for {
		<-modbusTicker.C
		rtu.GetHPPower(mbus)
		rtu.GetACPower(mbus)
		rtu.GetIO(mbus)
	}
}

// Get the data from the heat pump power monitor
func (rtu *ModbusRTUIO) GetHPPower(mbus *modbus.ModbusClient) {
	// Grab the modbus system
	rtu.muModbus.Lock()
	defer rtu.muModbus.Unlock()

	// Select the modbus client
	if err := mbus.SetUnitId(rtu.hpSlaveAddress); err != nil {
		// Log the error and drop out
		log.Print(err)
		return
	}
	// We need to sleep between changes in Unit ID
	time.Sleep(time.Millisecond * 50)
	// Read the values from the modbus client
	if result, err := mbus.ReadRegisters(VOLTAGEREGISTER, 10, modbus.INPUT_REGISTER); err != nil {
		// Log the error and drop out
		log.Println("Error reading from modbus slave at ", rtu.hpSlaveAddress, err)
		return
	} else {
		// Grab the buffer and save the values read from the modbus client
		rtu.muBuffer.Lock()
		defer rtu.muBuffer.Unlock()
		rtu.hpCurrent = getFloat32(result[CURRENTREGISTER:CURRENTREGISTER+CURRENTLENGTH+1]) / 1000
		rtu.hpVolts = float32(result[VOLTAGEREGISTER]) / 10
		rtu.hpEnergy = getFloat32(result[ENERGYREGISTER : ENERGYREGISTER+ENERGYLENGTH+1])
		rtu.hpFrequency = getFloat32(result[FREQUENCYREGISTER:FREQUENCYREGISTER+FREQUENCYLENGTH+1]) / 10
		rtu.hpPower = getFloat32(result[POWERREGISTER:POWERREGISTER+POWERLENGTH+1]) / 10
		rtu.hpPowerFactor = result[POWERFACTORREGISTER]
		rtu.lastHPUpdate = time.Now()
	}
}

// Get the data from the Firefly power monitor
func (rtu *ModbusRTUIO) GetACPower(mbus *modbus.ModbusClient) {
	// Grab the modbus system
	rtu.muModbus.Lock()
	defer rtu.muModbus.Unlock()

	if err := mbus.SetUnitId(rtu.acSlaveAddress); err != nil {
		// Log the error and drop out
		log.Print(err)
		return
	}
	// We need to sleep between changes in Unit ID
	time.Sleep(time.Millisecond * 50)
	if result, err := mbus.ReadRegisters(VOLTAGEREGISTER, 10, modbus.INPUT_REGISTER); err != nil {
		// Log the error and drop out
		log.Println(err)
		return
	} else {
		// Grab the buffer and save the values read from the modbus client
		rtu.muBuffer.Lock()
		defer rtu.muBuffer.Unlock()

		rtu.acCurrent = getFloat32(result[CURRENTREGISTER:CURRENTREGISTER+CURRENTLENGTH]) / 1000
		rtu.acVolts = float32(result[VOLTAGEREGISTER]) / 10
		rtu.acEnergy = getFloat32(result[ENERGYREGISTER : ENERGYREGISTER+ENERGYLENGTH])
		rtu.acFrequency = float32(result[FREQUENCYREGISTER]) / 10
		rtu.acPower = getFloat32(result[POWERREGISTER:POWERREGISTER+POWERLENGTH]) / 10
		rtu.acPowerFactor = result[POWERFACTORREGISTER]
		rtu.lastACUpdate = time.Now()
		//		log.Println("Energy = ", result)
	}
}

// Get the data from the Firefly io board
func (rtu *ModbusRTUIO) GetIO(mbus *modbus.ModbusClient) {
	var analogueInputs struct {
		conductivity     float32
		fuelCellPressure float32
		//		usused1	float32
		//		unused2 float32
		//		unused3 float32
		tankPressure         float64
		rawWaterConductivity uint16
		rawTankPressure      uint16
		rawFuelCellPressure  uint16
	}
	var coils []bool

	// Grab the modbus system
	rtu.muModbus.Lock()
	defer rtu.muModbus.Unlock()

	//		mbus.SetUnitId(uint8(*RelaySlaveAddress))
	if err := mbus.SetUnitId(rtu.relaySlaveAddress); err != nil {
		// Log the error and drop out
		log.Print(err)
		return
	}
	// We need to sleep between changes in Unit ID
	time.Sleep(time.Millisecond * 50)
	if err := mbus.SetEncoding(modbus.BIG_ENDIAN, modbus.LOW_WORD_FIRST); err != nil {
		// Log the error and drop out
		log.Print(err)
		return
	}
	coils, err := mbus.ReadCoils(1, 16)
	if err != nil {
		// Log the error and drop out
		log.Println("Modbus error:", err)
		return
	}
	input, err := mbus.ReadRegisters(1, 8, modbus.INPUT_REGISTER)
	if err != nil {
		// Log the error and drop out
		log.Println("Modbus error:", err)
		return
	}

	analogueInputs.rawWaterConductivity = input[CONDUCTIVITY-1]
	analogueInputs.rawTankPressure = input[TANKPRESSURE-1]
	analogueInputs.rawFuelCellPressure = input[FUELCELLPRESSURE-1]
	// convert to uS/cm * 10
	analogueInputs.conductivity = params.ConvertWaterConductivity(analogueInputs.rawWaterConductivity)
	// Convert fuel cell gas pressure to mBar * 10
	analogueInputs.fuelCellPressure = params.ConvertFuelCellPressure(input[FUELCELLPRESSURE-1])
	// Convert gas tank pressure to mBar
	analogueInputs.tankPressure = params.ConvertTankPressure(input[TANKPRESSURE-1])
	// log.Println("Tank = ", input[TANKPRESSURE-1], " = ", analogueInputs.tankPressure)
	rtu.muBuffer.Lock()
	defer rtu.muBuffer.Unlock()

	for idx, coil := range coils {
		switch idx + 1 {
		case RELAYFC0EN:
			rtu.fc0en = coil
		case RELAYFC0RUN:
			rtu.fc0run = coil
		case RELAYFC1EN:
			rtu.fc1en = coil
		case RELAYFC1RUN:
			rtu.fc1run = coil
		case RELAYEL0:
			rtu.el0 = coil
		case RELAYEL1:
			rtu.el1 = coil
		case RELAYGAS:
			rtu.gas = coil
		case RELAYSPARE:
			rtu.spare = coil
		}
	}
	rtu.conductivity = analogueInputs.conductivity
	rtu.fuelCellPressure = analogueInputs.fuelCellPressure
	rtu.tankPressure = analogueInputs.tankPressure
	rtu.rawConductivity = analogueInputs.rawWaterConductivity
	rtu.rawTankPressure = analogueInputs.rawTankPressure
	rtu.rawFuelCellPressure = analogueInputs.rawFuelCellPressure
	rtu.lastIOUpdate = time.Now()
}

func getFloat32(buffer []uint16) float32 {
	return float32(uint32(buffer[0]) + (uint32(buffer[1]) << 16))
}

func (rtu *ModbusRTUIO) getRelayStatus() {
	rtu.muBuffer.Lock()
	defer rtu.muBuffer.Unlock()
	SystemStatus.Relays.FC0Enable = rtu.fc0en
	SystemStatus.Relays.FC0Run = rtu.fc0run
	SystemStatus.Relays.FC1Enable = rtu.fc1en
	SystemStatus.Relays.FC1Run = rtu.fc1run
	SystemStatus.Relays.EL0 = rtu.el0
	SystemStatus.Relays.EL1 = rtu.el1
	SystemStatus.Relays.Spare = rtu.spare
	SystemStatus.Relays.GasToFuelCell = rtu.gas
}

func (rtu *ModbusRTUIO) getACStatus() {
	rtu.muBuffer.Lock()
	defer rtu.muBuffer.Unlock()
	SystemStatus.AC.ACCurrent = uint32(rtu.acCurrent * 100)
	SystemStatus.AC.ACPower = uint32(rtu.acPower * 100)
	SystemStatus.AC.ACFrequency = uint16(rtu.acFrequency * 100)
	SystemStatus.AC.ACVolts = uint16(rtu.acVolts * 100)
	SystemStatus.AC.ACPowerFactor = uint8(rtu.acPowerFactor)
	SystemStatus.AC.ACEnergy = uint32(rtu.acEnergy)
	SystemStatus.HP.ACCurrent = uint32(rtu.hpCurrent * 100)
	SystemStatus.HP.ACPower = uint32(rtu.hpPower * 100)
	SystemStatus.HP.ACFrequency = uint16(rtu.hpFrequency * 100)
	SystemStatus.HP.ACVolts = uint16(rtu.hpVolts * 100)
	SystemStatus.HP.ACPowerFactor = uint8(rtu.hpPowerFactor)
	SystemStatus.HP.ACEnergy = uint32(rtu.hpEnergy)
}

func (rtu *ModbusRTUIO) getGasStatus() {
	rtu.muBuffer.Lock()
	defer rtu.muBuffer.Unlock()
	SystemStatus.Gas.TankPressure = rtu.tankPressure
	SystemStatus.Gas.FuelCellPressure = rtu.fuelCellPressure
	SystemStatus.Gas.RawTankPressure = rtu.rawTankPressure
	SystemStatus.Gas.RawFuelCellPressure = rtu.rawFuelCellPressure
}

func (rtu *ModbusRTUIO) getTdsStatus() {
	rtu.muBuffer.Lock()
	defer rtu.muBuffer.Unlock()
	SystemStatus.TDS.RawTdsReading = rtu.rawConductivity
	SystemStatus.TDS.TdsReading = rtu.conductivity
}

func boolToOnOff(b bool) string {
	if b {
		return "ON"
	}
	return "OFF"
}
func (rtu *ModbusRTUIO) RelayOnOff(relay uint16, on bool) error {
	rtu.muModbus.Lock()
	defer rtu.muModbus.Unlock()
	if err := rtu.mbus.SetUnitId(rtu.relaySlaveAddress); err != nil {
		return err
	}
	debugPrint("Set relay %d to %s\n", relay, boolToOnOff(on))
	return rtu.mbus.WriteCoil(relay, on)
}

/*
GasOnOff ... turns on or off the gas solenoid feeding the fuel cells
*/
func (rtu *ModbusRTUIO) GasOnOff(on bool) error {
	return rtu.RelayOnOff(RELAYGAS, on)
}

/*
SpareOnOff turns on or off the spare solenoid
*/
func (rtu *ModbusRTUIO) SpareOnOff(on bool) error {
	return rtu.RelayOnOff(RELAYSPARE, on)
}

/*
EL0OnOff turns on or off the power to Electrolyser 0 and the dryer
*/
func (rtu *ModbusRTUIO) EL0OnOff(on bool) error {
	if !on && SystemStatus.ElectrolyserLock {
		// Just ignore the off command if we are locked
		return nil
	}
	return rtu.RelayOnOff(RELAYEL0, on)
}

/*
EL1OnOff turns on or off the power to Electrolyser 1
*/
func (rtu *ModbusRTUIO) EL1OnOff(on bool) error {
	if !on && SystemStatus.ElectrolyserLock {
		// Just ignore the off command if we are locked
		return nil
	}
	return rtu.RelayOnOff(RELAYEL1, on)
}

/*
FC0OnOff turns on or off the power to Fuel Cell 0
*/
func (rtu *ModbusRTUIO) FC0OnOff(on bool) error {
	return rtu.RelayOnOff(RELAYFC0EN, on)
}

/*
FC0RunStop turns on or off the run relay for Fuel Cell 0
*/
func (rtu *ModbusRTUIO) FC0RunStop(run bool) error {
	return rtu.RelayOnOff(RELAYFC0RUN, run)
}

/*
FC1OnOff turns on or off the power to Fuel Cell 1
*/
func (rtu *ModbusRTUIO) FC1OnOff(on bool) error {
	return rtu.RelayOnOff(RELAYFC1EN, on)
}

/*
FC1RunStop turns on or off the run relay for Fuel Cell 1
*/
func (rtu *ModbusRTUIO) FC1RunStop(run bool) error {
	return rtu.RelayOnOff(RELAYFC1RUN, run)
}

func (rtu *ModbusRTUIO) FCOnOff(device uint8, on bool) error {
	switch device {
	case 0:
		return rtu.FC0OnOff(on)
	case 1:
		return rtu.FC1OnOff(on)
	default:
		log.Printf("Invalid fuel cell (%d)", device)
		return fmt.Errorf("invalid fuel cell (%d)", device)
	}
}

func (rtu *ModbusRTUIO) FCRunStop(device uint8, run bool) error {
	switch device {
	case 0:
		return rtu.FC0RunStop(run)
	case 1:
		return rtu.FC1RunStop(run)
	default:
		log.Printf("Invalid fuel cell (%d)", device)
		return fmt.Errorf("invalid fuel cell (%d)", device)
	}
}

func (rtu *ModbusRTUIO) ELOnOff(device uint8, on bool) error {
	switch device {
	case 0:
		return rtu.EL0OnOff(on)
	case 1:
		return rtu.EL1OnOff(on)
	default:
		log.Printf("Invalid electrolyser (%d)", device)
		return fmt.Errorf("invalid electrolyser (%d)", device)
	}
}
