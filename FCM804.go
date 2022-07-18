package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"sync"
	"time"
)

//
//Fault Flag A bit field definitions
//
//
//const AnodeOverPressure = 0
//const AnodeUnderPressure = 1
//const Stack1OverCurrent = 2
//const Outlet1OverTemperature = 3
//const Stack1MinCellUndervoltage = 4
//const Inlet1OverTemperature = 5
//const SafetyObserverWatchdogTrip = 6
//const BoardOverTemperature = 7
//const SafetyObserverFanTrip = 8
//const ValveDefeatCheckFault = 9
//const Stack1UnderVoltage = 10
//const Stack1OverVoltage = 11
//const SafetyObserverMismatch = 12
//const Stack2MinCellUndervoltage = 13
//const SafetyObserverPressureTrip = 14
//const SafetyObserverBoardTxTrip = 15
//const Stack3MinCellUndervoltage = 16
//const SafetyObserverSoftwareTrip = 17
//const Fan2NoTacho = 18
//const Fan1NoTacho = 19
//const Fan3NoTacho = 20
//const Fan3ErrantSpeed = 21
//const Fan2ErrantSpeed = 22
//const Fan1ErrantSpeed = 23
//const Sib1Fault = 24
//const Sib2Fault = 25
//const Sib3Fault = 26
//const Inlet1TxSensorFault = 27
//const Outlet1TxSensorFault = 28
//const InvalidSerialNumber = 29
//const Dcdc1CurrentWhenDisabled = 30
//const Dcdc1OverCurrent = 31
//
///**
//Fault Flag B bit field definitions
//*/
//
//const AmbientOverTemperature = 0
//const Sib1CommsFault = 1
//const BoardTxSensorFault = 2
//const Sib2CommsFault = 3
//const LowLeakTestPressure = 4
//const Sib3CommsFault = 5
//const LouverOpenFault = 6
//const StateDependentUnexpectedCurrent1 = 7
//const EngineeringFault = 8
//const LowPurgeModifierIndicator = 9
//const Dcdc2CurrentWhenDisabled = 10
//const Dcdc3CurrentWhenDisabled = 11
//const Dcdc2OverCurrent = 12
//const ReadConfigFault = 13
//const CorruptConfigFault = 14
//const ConfigValueRangeFault = 15
//const Stack1VoltageMismatch = 16
//const Dcdc3OverCurrent = 17
//const UnexpectedPurgeInhibit = 18
//const FuelOnNoVolts = 19
//const LeakDetected = 20
//const AirCheckFault = 21
//const AirCheckFaultShadow = 22
//const DenyStartUV = 23
//const StateDependentUnexpectedCurrent2 = 24
//const StateDependentUnexpectedCurrent3 = 25
//const Stack2UnderVoltage = 26
//const Stack3UnderVoltage = 27
//const Stack2OverVoltage = 28
//const Stack3OverVoltage = 29
//const Stack2OverCurrent = 30
//const Stack3OverCurrent = 31
//
///**
//Fault Flag C bit field definitions
//*/
//
//const Stack2VoltageMismatch = 0
//const Stack3VoltageMismatch = 1
//const Outlet2OverTemperature = 2
//const Outlet3OverTemperature = 3
//const Inlet2OverTemperature = 4
//const Inlet3OverTemperature = 5
//const Inlet2TxSensorFault = 6
//const Inlet3TxSensorFault = 7
//const Outlet2TxSensorFault = 8
//const Outlet3TxSensorFault = 9
//const FuelOn1LowMeanVoltage = 10
//const FuelOn2LowMeanVoltage = 11
//const FuelOn3LowMeanVoltage = 12
//const FuelOn1LowMinVoltage = 13
//const FuelOn2LowMinVoltage = 14
//const FuelOn3LowMinVoltage = 15
//const SoftwareTripShutdown = 16
//const SoftwareTripFault = 17
//const TurnAroundTimeWarning = 18
//const PurgeCheckShutdown = 19
//const OutputUnderVoltage = 20
//const OutputOverVoltage = 21
//const SafetyObserverVoltRailTrip = 22
//const SafetyObserverDiffPressureTrip = 23
//const PurgeMissedOnePxOpen = 24
//const PurgeMissedOnePxClose = 25
//const PurgeMissedOneIxOpen = 26
//const PurgeMissedOneIxSolSaver = 27
//const PurgeMissedOneIxClose = 28
//const InRangeFaultPx01 = 29
//const NoisyInputPx01 = 30
//const NoisyInputTx68 = 31
//
///**
//Fault Flag D bit field definitions
//*/
//
//const NoisyInputDiffP = 0
//const ValveClosedPxRising = 1
//const DiffPSensorFault = 2
//const LossOfVentilation = 3
//const DiffPSensorHigh = 4
//const FanOverrun = 5
//const BlockedAirFlow = 6
//const WarningNoisyInputPx01 = 7
//const WarningNoisyInputTx68 = 8
//const WarningNoisyInputDiffP = 9
//const Dcdc1OutputFault = 10
//const EmergencyPurge = 11
//const EmergencyPurgeWarningA = 12
//const EmergencyPurgeWarningB = 13
//const EmergencyPurgeFault = 14
//const CalcCoreTxSensorFault = 15
//const CalcCoreOverTemperature = 16
//const LouverFailedToOpen = 17
//const LouverFailedToClose = 18
//const Dcdc2OutputFault = 19
//const Dcdc3OutputFault = 20
//const SidebySideTargetVoltagesShutdown = 21
//const SideBySideCanMessageFault = 22
//const SideBySideCanMessageIndicator = 23
//const AdcMonitorFault = 24
//const TachoIrqCounterFault = 25
//const TurnAroundTimeFault = 26
//
//// const - = 27 Not Used
//
//const Dcdc1ControlCheckFault = 28
//const Dcdc2ControlCheckFault = 29
//const Dcdc3ControlCheckFault = 30
//const I2c2DacsFault = 31
//
//// Bit mask to exclude Indicator fault flags
//// bit 42
//const IndicatorFaultA = ^uint32(0)
//const excludeIndicatorFaultA = ^IndicatorFaultA
//
//const IndicatorFaultB = ^uint32(1 << LowPurgeModifierIndicator)
//const excludeIndicatorFaultB = ^IndicatorFaultB
//
//// bit 82, 83, 89, 90, 91, 92, 93, 104, 105, 106, 115, 120
//const IndicatorFaultC = ^uint32((1 << SoftwareTripFault) |
//	(1 << TurnAroundTimeWarning) |
//	(1 << PurgeMissedOnePxOpen) |
//	(1 << PurgeMissedOnePxClose) |
//	(1 << PurgeMissedOneIxOpen) |
//	(1 << PurgeMissedOneIxSolSaver) |
//	(1 << PurgeMissedOneIxClose))
//const excludeIndicatorFaultC = ^IndicatorFaultC
//
//const IndicatorFaultD = ^uint32((1 << WarningNoisyInputPx01) |
//	(1 << WarningNoisyInputTx68) |
//	(1 << WarningNoisyInputDiffP) |
//	(1 << LouverFailedToClose) |
//	(1 << SideBySideCanMessageIndicator))
//const excludeIndicatorFaultD = ^IndicatorFaultD
//
//const ControlledFaultA = uint32(0)
//const excludeControlledFaultA = ^ControlledFaultA
//
//const ControlledFaultB = ^uint32(1 << DenyStartUV)
//const excludeControlledFaultB = ^ControlledFaultB
//
//const ControlledFaultC = ^uint32(0)
//const excludeControlledFaultC = ^ControlledFaultC
//
//const ControlledFaultD = uint32((1 << EmergencyPurge) |
//	(1 << EmergencyPurgeWarningA) |
//	(1 << EmergencyPurgeWarningB) |
//	(1 << EmergencyPurgeFault))
//const excludeControlledFaultD = ^ControlledFaultD
//
//const ShutdownFaultA = uint32((1 << AnodeUnderPressure) |
//	(1 << Stack1OverVoltage) |
//	(1 << Stack1MinCellUndervoltage) |
//	(1 << Stack1UnderVoltage) |
//	(1 << Stack1OverVoltage) |
//	(1 << Stack2MinCellUndervoltage) |
//	(1 << Stack3MinCellUndervoltage) |
//	(1 << Fan2NoTacho) |
//	(1 << Fan1NoTacho) |
//	(1 << Fan3NoTacho) |
//	(1 << Fan3ErrantSpeed) |
//	(1 << Fan2ErrantSpeed) |
//	(1 << Fan1ErrantSpeed) |
//	(1 << Sib1Fault) |
//	(1 << Sib2Fault) |
//	(1 << Sib3Fault) |
//	(1 << Inlet1TxSensorFault) |
//	(1 << Outlet1TxSensorFault) |
//	(1 << InvalidSerialNumber) |
//	(1 << Dcdc1CurrentWhenDisabled) |
//	(1 << Dcdc1OverCurrent))
//const excludeShutdownFaultA = ^ShutdownFaultA
//
//const ShutdownFaultB = uint32((1 << AmbientOverTemperature) |
//	(1 << Sib1CommsFault) |
//	(1 << BoardTxSensorFault) |
//	(1 << Sib2CommsFault) |
//	(1 << LowLeakTestPressure) |
//	(1 << Sib3CommsFault) |
//	(1 << LouverOpenFault) |
//	(1 << EngineeringFault) |
//	(1 << Dcdc2CurrentWhenDisabled) |
//	(1 << Dcdc3CurrentWhenDisabled) |
//	(1 << Dcdc2OverCurrent) |
//	(1 << ReadConfigFault) |
//	(1 << CorruptConfigFault) |
//	(1 << ConfigValueRangeFault) |
//	(1 << Stack1VoltageMismatch) |
//	(1 << Dcdc3OverCurrent) |
//	(1 << UnexpectedPurgeInhibit) |
//	(1 << FuelOnNoVolts) |
//	(1 << Stack2UnderVoltage) |
//	(1 << Stack3UnderVoltage))
//const excludeShutdownFaultB = ^ShutdownFaultB
//
//// bit 82, 83, 89, 90, 91, 92, 93, 104, 105, 106, 115, 120
//const ShutdownFaultC = uint32((1 << Stack2VoltageMismatch) |
//	(1 << Stack3VoltageMismatch) |
//	(1 << Inlet2TxSensorFault) |
//	(1 << Inlet3TxSensorFault) |
//	(1 << Outlet2TxSensorFault) |
//	(1 << Outlet3TxSensorFault) |
//	(1 << FuelOn1LowMeanVoltage) |
//	(1 << FuelOn2LowMeanVoltage) |
//	(1 << FuelOn3LowMeanVoltage) |
//	(1 << FuelOn1LowMinVoltage) |
//	(1 << FuelOn2LowMinVoltage) |
//	(1 << FuelOn3LowMinVoltage) |
//	(1 << SoftwareTripShutdown) |
//	(1 << PurgeCheckShutdown) |
//	(1 << OutputUnderVoltage))
//const excludeShutdownFaultC = ^ShutdownFaultC
//
//const ShutdownFaultD = uint32((1 << Dcdc1OutputFault) |
//	(1 << CalcCoreTxSensorFault) |
//	(1 << LouverFailedToOpen) |
//	(1 << Dcdc2OutputFault) |
//	(1 << Dcdc3OutputFault) |
//	(1 << SidebySideTargetVoltagesShutdown) |
//	(1 << SideBySideCanMessageIndicator) |
//	(1 << AdcMonitorFault) |
//	(1 << TachoIrqCounterFault) |
//	(1 << TurnAroundTimeFault) |
//	(1 << Dcdc1ControlCheckFault) |
//	(1 << Dcdc2ControlCheckFault) |
//	(1 << Dcdc3ControlCheckFault) |
//	(1 << I2c2DacsFault))
//const excludeShutdownFaultD = ^ShutdownFaultD
//
//const CriticalFaultA = ^(IndicatorFaultA | ControlledFaultA | ShutdownFaultA)
//const CriticalFaultB = ^(IndicatorFaultB | ControlledFaultB | ShutdownFaultB)
//const CriticalFaultC = ^(IndicatorFaultC | ControlledFaultC | ShutdownFaultC)
//const CriticalFaultD = ^(IndicatorFaultD | ControlledFaultD | ShutdownFaultD)
//

type FaultLevel int // Possible fault levels for the FCM804

const (
	None FaultLevel = iota
	Indicator
	Controlled
	Shutdown
	Critical
)

func (f FaultLevel) String() string {
	switch f {
	case None:
		return "None"
	case Indicator:
		return "Indicator"
	case Controlled:
		return "Controlled"
	case Shutdown:
		return "Shutdown"
	case Critical:
		return "Critical"
	default:
		return "Unknown"
	}
}

type FCM804 struct {
	bus         *CANBus
	device      uint16
	runState    bool      // Used to record the run state when restarting
	LastUpdate  time.Time // Serves as a heart beat
	FaultTime   time.Time // Time of the last fault on this cell
	ClearTime   time.Time // Time the fault was cleared
	InRestart   bool      // Are we restarting the cell?
	NumRestarts int       // How many restarts have we done to try and clear the current fault?
	Serial      [16]byte  // 2 0x310 frames sent sequentially. The second one has the most significant bit set
	Software    struct {  // 0x318 Firmware version on the Cell
		Major   int //byte-0
		Minor   int //byte-1
		Version int //byte-2
	}
	RunHours            uint32   // 0x320 byte 0..3
	RunEnergy           uint64   // 0x320 byte 4..7 * 20
	FaultA              uint32   // 0x328 byte 0..3
	FaultB              uint32   // 0x328 byte 4..7
	OutputPower         int16    // 0x338 byte 0..1 - Watts
	OutputVolts         float32  // 0x338 byte 2..3 - Volts * 0.01
	OutputCurrent       float32  // 0x338 byte 4..5 - Amps * 0.01
	AnodePressure       float32  // 0x338 byte 6..7 - mBar * 0.1
	OutletTemp          float32  // 0x348 byte 0..1 - C * 0.01
	InletTemp           float32  // 0x348 byte 2..3 - C * 0.01
	DCDCvoltageSetpoint float32  // 0x348 byte 4..5 - V * 0.01
	DCDCcurrentlimit    float32  // 0x348 byte 6..7 - A * 0.01
	LouverPosition      float32  // 0x358 byte 0..1 - %Open * 0.01
	FanSPduty           float32  // 0x358 byte 2..3 - % * 0.01
	StateInformation    struct { // 0x368 byte 0
		Inactive bool // bit7
		Run      bool // bit6
		Standby  bool // bit5
		Fault    bool // bit4
	}
	LoadLogic struct { // 0x368 byte 1
		DCDCDisabled bool // bit7
		OnLoad       bool // bit6
		FanPulse     bool // bit5
		Derated      bool // bit4
	}
	OutputBits struct { // 0x368 byte 2
		SV01              bool // bit7
		SV02              bool // bit6
		SV04              bool // bit5
		LouverOpen        bool // bit4
		DCDCEnabled       bool // bit3
		powerfromstack    bool // bit2
		powerfromexternal bool // bit1
	}
	FaultC uint32 // 0x378 byte 0..3
	FaultD uint32 // 0x378 byte 4..7

	mu sync.Mutex
}

func NewFCM804(bus *CANBus, device uint16) *FCM804 {
	fcm := new(FCM804)
	fcm.device = device
	fcm.bus = bus
	fcm.FaultA = 0
	fcm.FaultB = 0
	fcm.FaultC = 0
	fcm.FaultD = 0
	fcm.LastUpdate = time.Now().Add(time.Minute * -1)

	return fcm
}

// Clear resets all the can data prior to powering up the fuel cell
func (fcm *FCM804) Clear() {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	fcm.DCDCvoltageSetpoint = 0
	fcm.DCDCcurrentlimit = 0
	fcm.FaultD = 0
	fcm.FaultC = 0
	fcm.FaultB = 0
	fcm.FaultA = 0
	fcm.FaultTime = *new(time.Time)
	fcm.LouverPosition = 0
	fcm.FanSPduty = 0
	fcm.OutletTemp = 0
	fcm.InletTemp = 0
	fcm.InRestart = false
	fcm.AnodePressure = 0
	fcm.OutputPower = 0
	fcm.OutputVolts = 0
	fcm.OutputCurrent = 0
	fcm.StateInformation.Fault = false
	fcm.StateInformation.Run = false
	fcm.StateInformation.Standby = false
	fcm.StateInformation.Inactive = false
	fcm.LouverPosition = 0
	fcm.OutputBits.SV01 = false
	fcm.OutputBits.SV02 = false
	fcm.OutputBits.SV04 = false
	fcm.OutputBits.DCDCEnabled = false
	fcm.OutputBits.LouverOpen = false
	fcm.OutputBits.powerfromexternal = false
	fcm.OutputBits.powerfromstack = false
	fcm.LoadLogic.OnLoad = false
	fcm.LoadLogic.FanPulse = false
	fcm.LoadLogic.Derated = false
	fcm.LoadLogic.DCDCDisabled = false
}

// Read access to the fuel cell structure
func (fcm *FCM804) getLastUpdate() time.Time {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.FaultTime
}
func (fcm *FCM804) getClearTime() time.Time {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.ClearTime
}
func (fcm *FCM804) getInRestart() bool {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.InRestart
}
func (fcm *FCM804) getNumRestarts() int {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.NumRestarts
}
func (fcm *FCM804) getSerial() string {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return string(fcm.Serial[:])
}
func (fcm *FCM804) getMajor() int {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.Software.Major
}
func (fcm *FCM804) getMinor() int {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.Software.Minor
}
func (fcm *FCM804) getVersion() int {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.Software.Version
}

func (fcm *FCM804) getRunHours() uint32 {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.RunHours
}
func (fcm *FCM804) getRunEnergy() uint64 {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.RunEnergy
}
func (fcm *FCM804) getFaultA() uint32 {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.FaultA
}
func (fcm *FCM804) getFaultB() uint32 {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.FaultB
}
func (fcm *FCM804) getOutputPower() int16 {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.OutputPower
}
func (fcm *FCM804) getOutputVolts() float32 {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.OutputVolts
}
func (fcm *FCM804) getOutputCurrent() float32 {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.OutputCurrent
}
func (fcm *FCM804) getAnodePressure() float32 {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.AnodePressure
}
func (fcm *FCM804) getOutletTemp() float32 {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.OutletTemp
}
func (fcm *FCM804) getInletTemp() float32 {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.InletTemp
}
func (fcm *FCM804) getDCDCvoltageSetpoint() float32 {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.DCDCvoltageSetpoint
}
func (fcm *FCM804) getDCDCcurrentlimit() float32 {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.DCDCcurrentlimit
}
func (fcm *FCM804) getLouverPosition() float32 {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.LouverPosition
}
func (fcm *FCM804) getFanSPduty() float32 {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.FanSPduty
}
func (fcm *FCM804) getInactive() bool {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.StateInformation.Inactive
}
func (fcm *FCM804) getRun() bool {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.StateInformation.Run
}
func (fcm *FCM804) getStandby() bool {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.StateInformation.Standby
}
func (fcm *FCM804) getFault() bool {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.StateInformation.Fault
}

func (fcm *FCM804) getDCDCDisabled() bool {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.LoadLogic.DCDCDisabled
}
func (fcm *FCM804) getOnLoad() bool {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.LoadLogic.OnLoad
}
func (fcm *FCM804) getFanPulse() bool {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.LoadLogic.FanPulse
}
func (fcm *FCM804) getDerated() bool {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.LoadLogic.Derated
}

func (fcm *FCM804) getSV01() bool {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.OutputBits.SV01
}
func (fcm *FCM804) getSV02() bool {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.OutputBits.SV02
}
func (fcm *FCM804) getSV04() bool {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.OutputBits.SV04
}
func (fcm *FCM804) getLouverOpen() bool {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.OutputBits.LouverOpen
}
func (fcm *FCM804) getDCDCEnabled() bool {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.OutputBits.DCDCEnabled
}
func (fcm *FCM804) getpowerfromstack() bool {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.OutputBits.powerfromstack
}
func (fcm *FCM804) getpowerfromexternal() bool {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.OutputBits.powerfromexternal
}

func (fcm *FCM804) getFaultC() uint32 {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.FaultC
}
func (fcm *FCM804) getFaultD() uint32 {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return fcm.FaultD
}

//Frame310 contains the serial number
func (fcm *FCM804) Frame310(frame []byte) {
	offset := 0
	if (frame[0] & 0x80) == 0 {
		offset = 8
	}
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	for i, b := range frame {
		fcm.Serial[i+offset] = b & 0x7f
	}
	fcm.LastUpdate = time.Now()
}

// Frame318 contains Software Version Info
func (fcm *FCM804) Frame318(frame []byte) {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	fcm.Software.Major = int(frame[0])
	fcm.Software.Minor = int(frame[1])
	fcm.Software.Version = int(frame[2])
	fcm.LastUpdate = time.Now()
}

//Frame320 contains Run Hours and Energy
func (fcm *FCM804) Frame320(frame []byte) {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	fcm.RunHours = binary.BigEndian.Uint32(frame[0:4])
	fcm.RunEnergy = uint64(binary.BigEndian.Uint32(frame[4:8])) * 20
	fcm.LastUpdate = time.Now()
}

//Frame328 contains FaultA and FaultB : Returns true if the value has changed
func (fcm *FCM804) Frame328(frame []byte) (changed bool) {
	a := binary.BigEndian.Uint32(frame[0:4])
	b := binary.BigEndian.Uint32(frame[4:8])

	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	// Check to see if either fault has changed since the last 0x328 frame was received
	changed = ((a ^ fcm.FaultA) != 0) || ((b ^ fcm.FaultB) != 0)
	fcm.FaultA = a
	fcm.FaultB = b
	fcm.LastUpdate = time.Now()

	return
}

//Frame338 Output Power, Output Volts, Output Current, Anode Pressure
func (fcm *FCM804) Frame338(frame []byte) {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	fcm.OutputPower = int16(binary.BigEndian.Uint16(frame[0:2]))
	fcm.OutputVolts = float32(int16(binary.BigEndian.Uint16(frame[2:4]))) / 100.0
	fcm.OutputCurrent = float32(int16(binary.BigEndian.Uint16(frame[4:6]))) / 100.0
	fcm.AnodePressure = float32(binary.BigEndian.Uint16(frame[6:8])) / 10.0
	fcm.LastUpdate = time.Now()
}

//Frame348 Outlet Temp, Inlet Temp, DCDC voltage setpoint, DCDC current limit
func (fcm *FCM804) Frame348(frame []byte) {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	fcm.OutletTemp = float32(int16(binary.BigEndian.Uint16(frame[0:2]))) / 100.0
	fcm.InletTemp = float32(int16(binary.BigEndian.Uint16(frame[0:2]))) / 100.0
	fcm.DCDCvoltageSetpoint = float32(int16(binary.BigEndian.Uint16(frame[0:2]))) / 100.0
	fcm.DCDCcurrentlimit = float32(int16(binary.BigEndian.Uint16(frame[0:2]))) / 100.0
	fcm.LastUpdate = time.Now()
}

//Frame358 Louver Position, Fan SP Duty
func (fcm *FCM804) Frame358(frame []byte) {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	fcm.LouverPosition = float32(int16(binary.BigEndian.Uint16(frame[0:2]))) / 100.0
	fcm.FanSPduty = float32(int16(binary.BigEndian.Uint16(frame[2:4]))) / 100.0
	fcm.LastUpdate = time.Now()
}

//Frame368 State Information
func (fcm *FCM804) Frame368(frame []byte) (inFault bool) {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	inFault = false
	fcm.StateInformation.Inactive = (frame[0] & 0x80) != 0
	fcm.StateInformation.Run = (frame[0] & 0x40) != 0
	fcm.StateInformation.Standby = (frame[0] & 0x20) != 0
	// Are we changing to a fault condition here?
	bFault := (frame[0] & 0x10) != 0
	if bFault && !fcm.StateInformation.Fault {
		inFault = true
	}
	fcm.StateInformation.Fault = bFault

	fcm.LoadLogic.DCDCDisabled = (frame[1] & 0x80) != 0
	fcm.LoadLogic.OnLoad = (frame[1] & 0x40) != 0
	fcm.LoadLogic.FanPulse = (frame[1] & 0x20) != 0
	fcm.LoadLogic.Derated = (frame[1] & 0x10) != 0

	fcm.OutputBits.SV01 = (frame[2] & 0x80) != 0
	fcm.OutputBits.SV02 = (frame[2] & 0x40) != 0
	fcm.OutputBits.SV04 = (frame[2] & 0x20) != 0
	fcm.OutputBits.LouverOpen = (frame[2] & 0x10) != 0
	fcm.OutputBits.DCDCEnabled = (frame[2] & 0x08) != 0
	fcm.OutputBits.powerfromstack = (frame[2] & 0x04) != 0
	fcm.OutputBits.powerfromexternal = (frame[2] & 0x02) != 0
	fcm.LastUpdate = time.Now()
	return
}

//Frame378 contains FaultC and FaultD : Returns true if the value has changed
func (fcm *FCM804) Frame378(frame []byte) (changed bool) {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	c := binary.BigEndian.Uint32(frame[0:4])
	d := binary.BigEndian.Uint32(frame[4:8])

	// Check to see if either fault has changed since the last 0x378 frame was received
	changed = ((c ^ fcm.FaultC) != 0) || ((d ^ fcm.FaultD) != 0)
	fcm.FaultC = c
	fcm.FaultD = d
	fcm.LastUpdate = time.Now()
	return
}

//ProcessFrame decodes the given frame and records the details in the struct for this fuel cell
func (fcm *FCM804) ProcessFrame(ID uint32, data []byte) (triggerLogDump bool) {
	// These are the frames we are interrested in
	//	fmt.Printf("%04x\n", ID)
	triggerLogDump = false
	switch ID {
	case 0x310:
		fcm.Frame310(data)
	case 0x318:
		fcm.Frame318(data)
	case 0x320:
		fcm.Frame320(data)
	case 0x328:
		fcm.Frame328(data)
	case 0x338:
		fcm.Frame338(data)
	case 0x348:
		fcm.Frame348(data)
	case 0x358:
		fcm.Frame358(data)
	case 0x368:
		triggerLogDump = fcm.Frame368(data)
	case 0x378:
		fcm.Frame378(data)
	}
	return
}

//IsSwitchedOn returns true if the cell is transmitting information at least every half second.
func (fcm *FCM804) IsSwitchedOn() bool {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	return time.Now().Sub(fcm.LastUpdate) < (time.Millisecond * 500)
}

//GetState return a string describing the state of the fuel cell
func (fcm *FCM804) GetState() (state string) {
	if !fcm.IsSwitchedOn() {
		state = "Switched Off"
	} else {
		fcm.mu.Lock()
		defer fcm.mu.Unlock()
		if fcm.StateInformation.Run {
			state = state + "Run"
		}
		if fcm.StateInformation.Standby {
			if len(state) > 0 {
				state += " "
			}
			state += "Standby "
		}
		if fcm.StateInformation.Fault {
			if len(state) > 0 {
				state += " "
			}
			state = state + "Fault"
		}
		if fcm.StateInformation.Inactive {
			if len(state) > 0 {
				state += " "
			}
			state = state + "Inactive"
		}
	}
	return
}

func (fcm *FCM804) GetFaultLevel() (FaultLevel, bool) {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()
	fl, reboot := fcm.bus.getMaxFaultLevel(fcm.FaultA, fcm.FaultB, fcm.FaultC, fcm.FaultD)
	return FaultLevel(fl), reboot
}

/**
Returns a JSSON object containing all the current fuel cell errors decoded.
*/
func getFuelCellErrors(w http.ResponseWriter, _ *http.Request) {
	sSQL := `select date_format(l1.logged, "%Y-%m-%d %H:%i:%s") as logged
     , ifnull(Decodefault('A', l1.fc1FaultFlagA), "") as fc0faultA
     , ifnull(DecodeFault('B', l1.fc1FaultFlagB), "") as fc0faultB
     , ifnull(DecodeFault('C', l1.fc1FaultFlagC), "") as fc0faultC
     , ifnull(DecodeFault('D', l1.fc1FaultFlagD), "") as fc0faultD
     , ifnull(Decodefault('A', l1.fc2FaultFlagA), "") as fc1faultA
     , ifnull(DecodeFault('B', l1.fc2FaultFlagB), "") as fc1faultB
     , ifnull(DecodeFault('C', l1.fc2FaultFlagC), "") as fc1faultC
     , ifnull(DecodeFault('D', l1.fc2FaultFlagD), "") as fc1faultD
  from logging l1
  join logging l2 on l1.id = l2.id - 1
    and l1.logged > date_add(now(), interval -1 day)
    and l2.logged  > date_add(now(), interval -1 day)
	and ifnull(l1.fc1FaultFlagA, 0)
	  | ifnull(l1.fc1FaultFlagB, 0)
	  | ifnull(l1.fc1FaultFlagC, 0)
	  | ifnull(l1.fc1FaultFlagD, 0)
	  | ifnull(l1.fc2FaultFlagA, 0)
	  | ifnull(l1.fc2FaultFlagB, 0)
	  | ifnull(l1.fc2FaultFlagC, 0)
	  | ifnull(l1.fc2FaultFlagD, 0) <> 0
	and (ifnull(l1.fc1FaultFlagA, 0) ^ ifnull(l2.fc1FaultFlagA, 0)) |
	    (ifnull(l1.fc1FaultFlagB, 0) ^ ifnull(l2.fc1FaultFlagB, 0)) |
	    (ifnull(l1.fc1FaultFlagC, 0) ^ ifnull(l2.fc1FaultFlagC, 0)) |
	    (ifnull(l1.fc1FaultFlagD, 0) ^ ifnull(l2.fc1FaultFlagD, 0)) |
	    (ifnull(l1.fc2FaultFlagA, 0) ^ ifnull(l2.fc2FaultFlagA, 0)) |
	    (ifnull(l1.fc2FaultFlagB, 0) ^ ifnull(l2.fc2FaultFlagB, 0)) |
	    (ifnull(l1.fc2FaultFlagC, 0) ^ ifnull(l2.fc2FaultFlagC, 0)) |
	    (ifnull(l1.fc2FaultFlagD, 0) ^ ifnull(l2.fc2FaultFlagD, 0)) > 0
	order by logged desc`

	type Row struct {
		Logged    string `json:"logged"`
		FC1FaultA string `json:"fc0FaultA"`
		FC1FaultB string `json:"fc0FaultB"`
		FC1FaultC string `json:"fc0FaultC"`
		FC1FaultD string `json:"fc0FaultD"`
		FC2FaultA string `json:"fc1FaultA"`
		FC2FaultB string `json:"fc1FaultB"`
		FC2FaultC string `json:"fc1FaultC"`
		FC2FaultD string `json:"fc1FaultD"`
	}

	var results []*Row

	rows, err := pDB.Query(sSQL)
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
		if err := rows.Scan(&(row.Logged), &(row.FC1FaultA), &(row.FC1FaultB), &(row.FC1FaultC), &(row.FC1FaultD),
			&(row.FC2FaultA), &(row.FC2FaultB), &(row.FC2FaultC), &(row.FC2FaultD)); err != nil {
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

func getFuelCellDetail(w http.ResponseWriter, r *http.Request) {
	type Row struct {
		Logged        string
		AnodePressure float32
		Power         float32
		FaultA        string
		FaultB        string
		FaultC        string
		FaultD        string
		OutletTemp    float32
		InletTemp     float32
		Volts         float32
		Amps          float32
		Run           bool
		Inactive      bool
		Standby       bool
		Fault         bool
	}
	var results []*Row
	var cell int32

	// Set the returned type to application/json
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	from := vars["from"]
	device := vars["device"]
	switch device {
	case "1":
		cell = 0
	case "2":
		cell = 1
	default:
		err := fmt.Errorf("invalid device - %s", device)
		log.Println(err)
		ReturnJSONError(w, "FuelCell", err, http.StatusBadRequest, true)
		return
	}
	rows, err := pDB.Query(`select round(UNIX_TIMESTAMP(logged), 1), AnodePressure / 1000 as AnodePressure, Power,
			ifnull(DecodeFault('A', FaultA),'') as FaultA, ifnull(DecodeFault('B', FaultB),'') as FaultB,
			ifnull(DecodeFault('C', FaultC),'') as FaultC, ifnull(DecodeFault('D', FaultD),'') as FaultD,
			OutletTemp / 10 as OutletTemp, InletTemp / 10 as InletTemp, Volts / 10 as Volts, Amps / 10 as Amps,
			ifnull(Run, false) as Run, ifnull(Inactive, false) as Inactive, ifnull(Standby, false) as Standby, ifnull(Fault, false) as Fault
from FuelCell
where logged > ?
  and cell = ?
  limit 600`, from, cell)
	if err != nil {
		ReturnJSONError(w, "FuelCell", err, http.StatusInternalServerError, true)
		return
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err)
		}
	}()
	for rows.Next() {
		row := new(Row)
		if err := rows.Scan(&(row.Logged), &(row.AnodePressure), &(row.Power), &(row.FaultA), &(row.FaultB), &(row.FaultC), &(row.FaultD),
			&(row.OutletTemp), &(row.InletTemp), &(row.Volts), &(row.Amps), &(row.Run), &(row.Inactive), &(row.Standby), &(row.Fault)); err != nil {
			ReturnJSONError(w, "FuelCell", err, http.StatusInternalServerError, true)
			return
		} else {
			results = append(results, row)
		}
	}
	if len(results) == 0 {
		ReturnJSONErrorString(w, "FuelCell", "No results found - "+from+" | "+device, http.StatusBadRequest, true)
		return
	}
	if JSON, err := json.Marshal(results); err != nil {
		ReturnJSONError(w, "FuelCell", err, http.StatusInternalServerError, true)
	} else {
		if _, err := fmt.Fprintf(w, string(JSON)); err != nil {
			log.Println(err)
		}
	}
}

func (fcm *FCM804) restartTheFuelCell() {
	if fcm.runState {
		if err := startFuelCell(int64(fcm.device)); err != nil {
			log.Println(err)
		}
	} else {
		if err := turnOnFuelCell(int64(fcm.device)); err != nil {
			log.Println(err)
		}
	}
}

/**
Check the fuel cell to see if there are any errors and reset it if there are
*/
func (fcm *FCM804) checkFuelCell() {

	//// Ignore checks.
	//return
	//	log.Println("Checking fuel cell ", device)
	if !fcm.InRestart {
		//		log.Println("Not in restart...")
		// We are not in a restart so check for faults.
		if fcm.StateInformation.Fault {
			//			log.Printf("Errors found |%s|%s|%s|%s|\n", fc.FaultFlagA, fc.FaultFlagB, fc.FaultFlagC, fc.FaultFlagD)
			// There is a fault so check the time
			if fcm.FaultTime == *new(time.Time) {
				// Time is blank so record the time and log the fault
				fcm.FaultTime = time.Now()
				log.Printf("Fuel Cell %d Fault : %08x|%08x|%08x|%08x", fcm.device, fcm.FaultA, fcm.FaultB, fcm.FaultC, fcm.FaultD)
			} else {
				// how long has the fault been present?
				t := time.Now().Add(0 - time.Minute)
				// If it has been more than a minute, and we have only logged MAXFUELCELLRESTARTS faults,
				// then restart the fuel cell.
				if fcm.FaultTime.Before(t) && fcm.NumRestarts < MAXFUELCELLRESTARTS {
					log.Println("Restarting fuel cell ", fcm.device)
					fcm.InRestart = true
					fcm.NumRestarts++
					if fcm.device == 0 {
						fcm.runState = SystemStatus.Relays.FuelCell1Run
					} else {
						fcm.runState = SystemStatus.Relays.FuelCell2Run
					}
					go func() {
						if err := turnOffFuelCell(int64(fcm.device)); err != nil {
							log.Println(err)
						}
					}()

					time.AfterFunc(OFFTIMEFORFUELCELLRESTART, func() {
						fcm.restartTheFuelCell()
					})
					err := smtp.SendMail("smtp.titan.email:587",
						smtp.PlainAuth("", "pi@cedartechnology.com", "7444561", "smtp.titan.email"),
						"pi@cedartechnology.com", []string{"ian.abercrombie@cedartechnology.com"}, []byte(`From: Aberhome1
To: Ian.Abercrombie@cedartechnology.com
Subject: Fuelcell Error encountered
The fuel cell has reported an error. I am attempting to restart it.
Fault A = `+strings.Join(getFuelCellError('A', fcm.FaultA), " : ")+`
Fault B = `+strings.Join(getFuelCellError('B', fcm.FaultB), " : ")+`
Fault C = `+strings.Join(getFuelCellError('C', fcm.FaultC), " : ")+`
Fault D = `+strings.Join(getFuelCellError('D', fcm.FaultD), " : ")+`
Restart number = `+strconv.Itoa(fcm.NumRestarts)))
					if err != nil {
						log.Println(err)
					}
				}
				fcm.ClearTime = *new(time.Time)
			}
		} else {
			// Clear the fault time set the clear time if it is blank
			fcm.FaultTime = *new(time.Time)
			if fcm.ClearTime == fcm.FaultTime {
				fcm.ClearTime = time.Now()
			} else {
				// If clear time is not blank, but it has been 5 minutes since we saw the clear then set the numRestarts to 0 and clear the time
				if time.Since(fcm.ClearTime) > (time.Minute * 5) {
					fcm.NumRestarts = 0
					fcm.ClearTime = fcm.FaultTime
				}
			}
		}
	}
}
