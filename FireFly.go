package main

/*****************************************
This project uses the firefly esm command line interface to control the system components.

*/

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"io"
	"log"
	"math"
	"net"
	"runtime"

	//	"log/syslog"
	syslog "github.com/RackSec/srslog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
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
const GASOFFDELAY = time.Second * 2
const convertPSIToBar = 14.503773773
const CANDUMPSQL = `SELECT UNIX_TIMESTAMP(logged) AS logged, Cell
     , data00, data00offsetms, data01, data01offsetms, data02, data02offsetms, data03, data03offsetms, data04, data04offsetms
     , data05, data05offsetms, data06, data06offsetms, data07, data07offsetms, data08, data08offsetms, data09, data09offsetms
     , data0A, data0Aoffsetms, data0B, data0Boffsetms, data0C, data0Coffsetms, data0D, data0Doffsetms, data0E, data0Eoffsetms
     , data0F, data0Foffsetms, data10, data10offsetms, data11, data11offsetms, data12, data12offsetms, data13, data13offsetms
     , data14, data14offsetms, data15, data15offsetms, data16, data16offsetms, data17, data17offsetms, data18, data18offsetms
     , data19, data19offsetms, data1A, data1Aoffsetms, data1B, data1Boffsetms, data1C, data1Coffsetms, data1D, data1Doffsetms
     , data1E, data1Eoffsetms, data1F, data1Foffsetms, data20, data20offsetms, data21, data21offsetms, data22, data22offsetms
     , data23, data23offsetms, data24, data24offsetms, data25, data25offsetms, data26, data26offsetms, data27, data27offsetms
     , data28, data28offsetms, data29, data29offsetms, data2A, data2Aoffsetms, data2B, data2Boffsetms, data2C, data2Coffsetms
     , data2D, data2Doffsetms, data2E, data2Eoffsetms
FROM firefly.CAN_Trace
 WHERE logged BETWEEN ? AND ?`
const CANDUMPEVENTSQL = `SELECT UNIX_TIMESTAMP(logged) AS logged, Cell
     , data00, data00offsetms, data01, data01offsetms, data02, data02offsetms, data03, data03offsetms, data04, data04offsetms
     , data05, data05offsetms, data06, data06offsetms, data07, data07offsetms, data08, data08offsetms, data09, data09offsetms
     , data0A, data0Aoffsetms, data0B, data0Boffsetms, data0C, data0Coffsetms, data0D, data0Doffsetms, data0E, data0Eoffsetms
     , data0F, data0Foffsetms, data10, data10offsetms, data11, data11offsetms, data12, data12offsetms, data13, data13offsetms
     , data14, data14offsetms, data15, data15offsetms, data16, data16offsetms, data17, data17offsetms, data18, data18offsetms
     , data19, data19offsetms, data1A, data1Aoffsetms, data1B, data1Boffsetms, data1C, data1Coffsetms, data1D, data1Doffsetms
     , data1E, data1Eoffsetms, data1F, data1Foffsetms, data20, data20offsetms, data21, data21offsetms, data22, data22offsetms
     , data23, data23offsetms, data24, data24offsetms, data25, data25offsetms, data26, data26offsetms, data27, data27offsetms
     , data28, data28offsetms, data29, data29offsetms, data2A, data2Aoffsetms, data2B, data2Boffsetms, data2C, data2Coffsetms
     , data2D, data2Doffsetms, data2E, data2Eoffsetms
FROM firefly.CAN_Trace
 WHERE Event = ?`
const LISTCANEVENTSSQL = `SELECT DISTINCT Event, OnDemand FROM CAN_Trace WHERE Event IS NOT NULL ORDER BY Event DESC LIMIT 50`
const redirectToMainMenuScript = `
<script>
	var tID = setTimeout(function () {
		window.location.href = "/status";
		window.clearTimeout(tID);		// clear time out.
	}, 2000);
</script>
`
const SUCCESSJSONRESPONCE = `{"success":true}`

type OnOffPayload struct {
	Device uint8 `json:"device"`
	State  bool  `json:"state"`
}

/*
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
*/

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
	FuelCellPressure    float32
	TankPressure        float64
	RawFuelCellPressure uint16
	RawTankPressure     uint16
}

type acStatus struct {
	ACPower       uint32 `json:"power"`
	ACCurrent     uint32 `json:"current"`
	ACVolts       uint16 `json:"volts"`
	ACPowerFactor uint8  `json:"powerfactor"`
	ACFrequency   uint16 `json:"frequency"`
	ACEnergy      uint32 `json:"energy"`
}

type relayStatus struct {
	FC0Enable     bool
	FC0Run        bool
	FC1Enable     bool
	FC1Run        bool
	Spare         bool
	EL0           bool
	EL1           bool
	GasToFuelCell bool
}

type tdsStatus struct {
	TdsReading    float32
	RawTdsReading uint16
}

var (
	databaseServer   string
	databasePort     string
	databaseName     string
	databaseLogin    string
	databasePassword string
	CANInterface     string
	pDB              *sql.DB

	SystemStatus struct {
		m                sync.Mutex
		valid            bool
		Relays           relayStatus
		Electrolysers    []*Electrolyser
		ElectrolyserLock bool // Prevents electrolysers from being turned off when set
		Gas              gasStatus
		TDS              tdsStatus
		AC               acStatus
		HP               acStatus
	}
	electrolyserShutDownTime time.Time

	jsonSettings string
	params       *JsonSettings

	canBus  *CANBus
	mbusRTU *ModbusRTUIO
)

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

func returnJSONSuccess(w http.ResponseWriter) {
	if _, err := fmt.Fprintf(w, SUCCESSJSONRESPONCE); err != nil {
		log.Println(err)
	}
}

/**
getGasHtmlStatus : return the html rendering of the Gas status from the gasStatus object
*/
func getGasHtmlStatus() (html string) {

	html = fmt.Sprintf(`<table>
  <tr><td class="label">Fuel Cell Pressure</td><td>%0.2f mbar</td><td class="label">Tank Pressure</td><td>%0.1f bar</td></tr>
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
<th class="%s">Fuel Cell 2 Enable</th><th class="%s">Fuel Cell 2 Run</th><th class="%s">Spare</th></tr></table>`,
		booleanToHtmlClass(SystemStatus.Relays.EL0),
		booleanToHtmlClass(SystemStatus.Relays.EL1),
		booleanToHtmlClass(SystemStatus.Relays.GasToFuelCell),
		booleanToHtmlClass(SystemStatus.Relays.FC0Enable),
		booleanToHtmlClass(SystemStatus.Relays.FC0Run),
		booleanToHtmlClass(SystemStatus.Relays.FC1Enable),
		booleanToHtmlClass(SystemStatus.Relays.FC1Run),
		booleanToHtmlClass(SystemStatus.Relays.Spare))
}

/**
getTdsHtmlStatus : return the html rendering of the Gas status from the gasStatus object
*/
func getTdsHtmlStatus() (html string) {

	html = fmt.Sprintf(`<table>
  <tr><td class="label">Total Dissolved Solids</td><td>%0.1f ppm</td></tr>
</table>`, SystemStatus.TDS.TdsReading)
	return html
}

/**
getStatus : return tha status page showing the complete system status
*/
func getStatus(w http.ResponseWriter, _ *http.Request) {

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
	bytesArray, err := json.Marshal(&SystemStatus.Electrolysers[device].status)
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
	mbusRTU.getRelayStatus()
	mbusRTU.getACStatus()

	SystemStatus.m.Lock()
	defer SystemStatus.m.Unlock()

	mbusRTU.getGasStatus()
	mbusRTU.getTdsStatus()

	for device := range SystemStatus.Electrolysers {

		ElectrolyserOn := false
		// Check the power relay to see if this electrolyser has power
		switch device {
		case 0:
			ElectrolyserOn = SystemStatus.Relays.EL0
		case 1:
			ElectrolyserOn = SystemStatus.Relays.EL1
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
		el0Rate             sql.NullInt16
		el0ElectrolyteLevel sql.NullByte
		el0ElectrolyteTemp  sql.NullInt16
		el0State            sql.NullByte
		el0H2Flow           sql.NullInt16
		el0H2InnerPressure  sql.NullInt16
		el0H2OuterPressure  sql.NullInt16
		el0StackVoltage     sql.NullInt16
		el0StackCurrent     sql.NullInt16
		el0SystemState      sql.NullByte
		el0WaterPressure    sql.NullInt16

		el1Rate             sql.NullInt16
		el1ElectrolyteLevel sql.NullByte
		el1ElectrolyteTemp  sql.NullInt16
		el1State            sql.NullByte
		el1H2Flow           sql.NullInt16
		el1H2InnerPressure  sql.NullInt16
		el1H2OuterPressure  sql.NullInt16
		el1StackVoltage     sql.NullInt16
		el1StackCurrent     sql.NullInt16
		el1SystemState      sql.NullByte
		el1WaterPressure    sql.NullInt16

		drTemp0          sql.NullInt16
		drTemp1          sql.NullInt16
		drTemp2          sql.NullInt16
		drTemp3          sql.NullInt16
		drInputPressure  sql.NullInt16
		drOutputPressure sql.NullInt16
		drWarning        sql.NullString
		drError          sql.NullString

		fc0State         sql.NullByte
		fc0AnodePressure sql.NullInt16
		fc0FaultFlagA    uint32
		fc0FaultFlagB    uint32
		fc0FaultFlagC    uint32
		fc0FaultFlagD    uint32
		fc0InletTemp     sql.NullInt16
		fc0OutletTemp    sql.NullInt16
		fc0OutputPower   sql.NullInt16
		fc0OutputCurrent sql.NullInt16
		fc0OutputVoltage sql.NullInt16

		fc1State         sql.NullByte
		fc1AnodePressure sql.NullInt16
		fc1FaultFlagA    uint32
		fc1FaultFlagB    uint32
		fc1FaultFlagC    uint32
		fc1FaultFlagD    uint32
		fc1InletTemp     sql.NullInt16
		fc1OutletTemp    sql.NullInt16
		fc1OutputPower   sql.NullInt16
		fc1OutputCurrent sql.NullInt16
		fc1OutputVoltage sql.NullInt16
	}

	SystemStatus.m.Lock()
	defer SystemStatus.m.Unlock()

	params.el0Rate.Valid = false
	params.el0ElectrolyteLevel.Valid = false
	params.el0ElectrolyteTemp.Valid = false
	params.el0State.Valid = false
	params.el0H2Flow.Valid = false
	params.el0H2InnerPressure.Valid = false
	params.el0H2OuterPressure.Valid = false
	params.el0StackVoltage.Valid = false
	params.el0StackCurrent.Valid = false
	params.el0SystemState.Valid = false
	params.el0WaterPressure.Valid = false

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

	params.drTemp0.Valid = false
	params.drTemp1.Valid = false
	params.drTemp2.Valid = false
	params.drTemp3.Valid = false
	params.drInputPressure.Valid = false
	params.drOutputPressure.Valid = false
	params.drWarning.Valid = false
	params.drError.Valid = false

	params.fc0State.Valid = false
	params.fc0AnodePressure.Valid = false
	params.fc0InletTemp.Valid = false
	params.fc0OutletTemp.Valid = false
	params.fc0OutputCurrent.Valid = false
	params.fc0OutputVoltage.Valid = false
	params.fc1State.Valid = false
	params.fc1AnodePressure.Valid = false
	params.fc1InletTemp.Valid = false
	params.fc1OutletTemp.Valid = false
	params.fc1OutputCurrent.Valid = false
	params.fc1OutputVoltage.Valid = false

	if len(SystemStatus.Electrolysers) > 0 {
		if SystemStatus.Relays.EL0 {
			params.el0SystemState.Byte = uint8(SystemStatus.Electrolysers[0].status.SystemState)
			params.el0SystemState.Valid = true
			params.el0ElectrolyteLevel.Byte = byte(SystemStatus.Electrolysers[0].status.ElectrolyteLevel)
			params.el0ElectrolyteLevel.Valid = true
			params.el0H2Flow.Int16 = int16(SystemStatus.Electrolysers[0].status.H2Flow * 10)
			params.el0H2Flow.Valid = true
			params.el0ElectrolyteTemp.Int16 = int16(SystemStatus.Electrolysers[0].status.ElectrolyteTemp * 10)
			params.el0ElectrolyteTemp.Valid = true
			params.el0State.Byte = uint8(SystemStatus.Electrolysers[0].status.ElState)
			params.el0State.Valid = true
			params.el0H2InnerPressure.Int16 = int16(SystemStatus.Electrolysers[0].status.InnerH2Pressure * 10)
			params.el0H2InnerPressure.Valid = true
			params.el0H2OuterPressure.Int16 = int16(SystemStatus.Electrolysers[0].status.OuterH2Pressure * 10)
			params.el0H2OuterPressure.Valid = true
			params.el0Rate.Int16 = int16(SystemStatus.Electrolysers[0].GetRate() * 10)
			params.el0Rate.Valid = true
			params.el0StackVoltage.Int16 = int16(SystemStatus.Electrolysers[0].status.StackVoltage * 10)
			params.el0StackVoltage.Valid = true
			params.el0StackCurrent.Int16 = int16(SystemStatus.Electrolysers[0].status.StackCurrent * 10)
			params.el0StackCurrent.Valid = true
			params.el0WaterPressure.Int16 = int16(SystemStatus.Electrolysers[0].status.WaterPressure * 10)
			params.el0WaterPressure.Valid = true

		} else {
			params.el0SystemState.Byte = 0xff
			params.el0SystemState.Valid = true
		}
	}
	if len(SystemStatus.Electrolysers) > 1 {
		if SystemStatus.Relays.EL1 {
			params.el1SystemState.Byte = uint8(SystemStatus.Electrolysers[1].status.SystemState)
			params.el1SystemState.Valid = true
			params.el1ElectrolyteLevel.Byte = byte(SystemStatus.Electrolysers[1].status.ElectrolyteLevel)
			params.el1ElectrolyteLevel.Valid = true
			params.el1H2Flow.Int16 = int16(SystemStatus.Electrolysers[1].status.H2Flow * 10)
			params.el1H2Flow.Valid = true
			params.el1ElectrolyteTemp.Int16 = int16(SystemStatus.Electrolysers[1].status.ElectrolyteTemp * 10)
			params.el1ElectrolyteTemp.Valid = true
			params.el1State.Byte = uint8(SystemStatus.Electrolysers[1].status.ElState)
			params.el1State.Valid = true
			params.el1H2InnerPressure.Int16 = int16(SystemStatus.Electrolysers[1].status.InnerH2Pressure * 10)
			params.el1H2InnerPressure.Valid = true
			params.el1H2OuterPressure.Int16 = int16(SystemStatus.Electrolysers[1].status.OuterH2Pressure * 10)
			params.el1H2OuterPressure.Valid = true
			params.el1Rate.Int16 = int16(SystemStatus.Electrolysers[1].GetRate() * 10)
			params.el1Rate.Valid = true
			params.el1StackVoltage.Int16 = int16(SystemStatus.Electrolysers[1].status.StackVoltage * 10)
			params.el1StackVoltage.Valid = true
			params.el1StackCurrent.Int16 = int16(SystemStatus.Electrolysers[1].status.StackCurrent * 10)
			params.el1StackCurrent.Valid = true
			params.el1WaterPressure.Int16 = int16(SystemStatus.Electrolysers[1].status.WaterPressure * 10)
			params.el1WaterPressure.Valid = true
		} else {
			params.el1SystemState.Byte = 0xff
			params.el1SystemState.Valid = true
		}
	}
	if len(SystemStatus.Electrolysers) > 0 {
		if SystemStatus.Relays.EL0 {
			params.drInputPressure.Int16 = int16(SystemStatus.Electrolysers[0].status.DryerInputPressure * 10)
			params.drInputPressure.Valid = true
			params.drOutputPressure.Int16 = int16(SystemStatus.Electrolysers[0].status.DryerOutputPressure * 10)
			params.drOutputPressure.Valid = true
			params.drTemp0.Int16 = int16(SystemStatus.Electrolysers[0].status.DryerTemp1 * 10)
			params.drTemp0.Valid = true
			params.drTemp1.Int16 = int16(SystemStatus.Electrolysers[0].status.DryerTemp2 * 10)
			params.drTemp1.Valid = true
			params.drTemp2.Int16 = int16(SystemStatus.Electrolysers[0].status.DryerTemp3 * 10)
			params.drTemp2.Valid = true
			params.drTemp3.Int16 = int16(SystemStatus.Electrolysers[0].status.DryerTemp4 * 10)
			params.drTemp3.Valid = true
			params.drWarning.String = SystemStatus.Electrolysers[0].GetDryerWarningText()
			params.drWarning.Valid = true
			params.drError.String = SystemStatus.Electrolysers[0].GetDryerErrorText()
			params.drError.Valid = true
		}
	}
	if fc, found := canBus.fuelCell[0]; found {
		if SystemStatus.Relays.FC0Enable {
			params.fc0AnodePressure.Int16 = int16(fc.AnodePressure) // millibar x 10
			params.fc0AnodePressure.Valid = true
			params.fc0FaultFlagA = fc.getFaultA()
			params.fc0FaultFlagB = fc.getFaultB()
			params.fc0FaultFlagC = fc.getFaultC()
			params.fc0FaultFlagD = fc.getFaultD()
			params.fc0InletTemp.Int16 = int16(fc.getInletTemp() * 10)
			params.fc0InletTemp.Valid = true
			params.fc0OutletTemp.Int16 = int16(fc.getOutletTemp() * 10)
			params.fc0OutletTemp.Valid = true
			params.fc0OutputCurrent.Int16 = int16(fc.getOutputCurrent() * 100)
			params.fc0OutputCurrent.Valid = true
			params.fc0OutputVoltage.Int16 = int16(fc.getOutputVolts() * 10)
			params.fc0OutputVoltage.Valid = true
			params.fc0OutputPower.Int16 = fc.getOutputPower()
			params.fc0OutputPower.Valid = true
			params.fc0State.Byte = fc.GetStateCode()
			params.fc0State.Valid = true
		} else {
			params.fc0State.Byte = 0
			params.fc0State.Valid = true
		}
	}
	if fc, found := canBus.fuelCell[1]; found {
		if SystemStatus.Relays.FC1Enable {
			params.fc1AnodePressure.Int16 = int16(fc.AnodePressure) // millibar
			params.fc1AnodePressure.Valid = true
			params.fc1FaultFlagA = fc.getFaultA()
			params.fc1FaultFlagB = fc.getFaultB()
			params.fc1FaultFlagC = fc.getFaultC()
			params.fc1FaultFlagD = fc.getFaultD()
			params.fc1InletTemp.Int16 = int16(fc.getInletTemp() * 10)
			params.fc1InletTemp.Valid = true
			params.fc1OutletTemp.Int16 = int16(fc.getOutletTemp() * 10)
			params.fc1OutletTemp.Valid = true
			params.fc1OutputCurrent.Int16 = int16(fc.getOutputCurrent() * 10)
			params.fc1OutputCurrent.Valid = true
			params.fc1OutputVoltage.Int16 = int16(fc.getOutputVolts() * 10)
			params.fc1OutputVoltage.Valid = true
			params.fc1OutputPower.Int16 = fc.getOutputPower() * 10
			params.fc1OutputPower.Valid = true
			params.fc1State.Byte = fc.GetStateCode()
			params.fc1State.Valid = true
		} else {
			params.fc1State.Byte = 0
			params.fc1State.Valid = true
		}
	}

	strCommand := `INSERT INTO firefly.logging(
            el0Rate, el0ElectrolyteLevel, el0ElectrolyteTemp, el0StateCode, el0H2Flow, el0H2InnerPressure, el0H2OuterPressure, el0StackVoltage, el0StackCurrent, el0SystemStateCode, el0WaterPressure, 
            drTemp0, drTemp1, drTemp2, drTemp3, drInputPressure, drOutputPressure, drWarning, drError, 
            el1Rate, el1ElectrolyteLevel, el1ElectrolyteTemp, el1StateCode, el1H2Flow, el1H2InnerPressure, el1H2OuterPressure, el1StackVoltage, el1StackCurrent, el1SystemStateCode, el1WaterPressure,
            fc0State, fc0AnodePressure, fc0FaultFlagA, fc0FaultFlagB, fc0FaultFlagC, fc0FaultFlagD, fc0InletTemp, fc0OutletTemp, fc0OutputPower, fc0OutputCurrent, fc0OutputVoltage,
            fc1State, fc1AnodePressure, fc1FaultFlagA, fc1FaultFlagB, fc1FaultFlagC, fc1FaultFlagD, fc1InletTemp, fc1OutletTemp, fc1OutputPower, fc1OutputCurrent, fc1OutputVoltage,
            gasFuelCellPressure, gasTankPressure,
            totalDissolvedSolids,
            relayGas, relayFuelCell0Enable, relayFuelCell0Run, relayFuelCell1Enable, relayFuelCell1Run, relayEl0Power, relayEl1Power, relaySpare,
            ACPower, ACVolts, ACCurrent, ACFrequency, ACPowerFactor, ACEnergy,
            HPPower, HPVolts, HPCurrent, HPFrequency, HPPowerFactor, HPEnergy)
	VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
	       ?, ?, ?, ?, ?, ?, ?, ?,
	       ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
	       ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
	       ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
	       ?, ?,
	       ?,
	       ?, ?, ?, ?, ?, ?, ?, ?,
	       ?, ?, ?, ?, ?, ?,
	       ?, ?, ?, ?, ?, ?);`

	_, err = pDB.Exec(strCommand,
		params.el0Rate, params.el0ElectrolyteLevel, params.el0ElectrolyteTemp, params.el0State, params.el0H2Flow, params.el0H2InnerPressure, params.el0H2OuterPressure, params.el0StackVoltage, params.el0StackCurrent, params.el0SystemState, params.el0WaterPressure,
		params.drTemp0, params.drTemp1, params.drTemp2, params.drTemp3, params.drInputPressure, params.drOutputPressure, params.drWarning, params.drError,
		params.el1Rate, params.el1ElectrolyteLevel, params.el1ElectrolyteTemp, params.el1State, params.el1H2Flow, params.el1H2InnerPressure, params.el1H2OuterPressure, params.el1StackVoltage, params.el1StackCurrent, params.el1SystemState, params.el1WaterPressure,
		params.fc0State, params.fc0AnodePressure, params.fc0FaultFlagA, params.fc0FaultFlagB, params.fc0FaultFlagC, params.fc0FaultFlagD, params.fc0InletTemp, params.fc0OutletTemp, params.fc0OutputPower, params.fc0OutputCurrent, params.fc0OutputVoltage,
		params.fc1State, params.fc1AnodePressure, params.fc1FaultFlagA, params.fc1FaultFlagB, params.fc1FaultFlagC, params.fc1FaultFlagD, params.fc1InletTemp, params.fc1OutletTemp, params.fc1OutputPower, params.fc1OutputCurrent, params.fc1OutputVoltage,
		SystemStatus.Gas.RawFuelCellPressure, SystemStatus.Gas.RawTankPressure,
		SystemStatus.TDS.RawTdsReading,
		SystemStatus.Relays.GasToFuelCell, SystemStatus.Relays.FC0Enable, SystemStatus.Relays.FC0Run, SystemStatus.Relays.FC1Enable, SystemStatus.Relays.FC1Run, SystemStatus.Relays.EL0, SystemStatus.Relays.EL1, SystemStatus.Relays.Spare,
		SystemStatus.AC.ACPower, SystemStatus.AC.ACVolts, SystemStatus.AC.ACCurrent, SystemStatus.AC.ACFrequency, SystemStatus.AC.ACPowerFactor, SystemStatus.AC.ACEnergy,
		SystemStatus.HP.ACPower, SystemStatus.HP.ACVolts, SystemStatus.HP.ACCurrent, SystemStatus.HP.ACFrequency, SystemStatus.HP.ACPowerFactor, SystemStatus.HP.ACEnergy)

	if err != nil {
		log.Printf("Error writing values to the database - %s", err)
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
		Gas           float64
	}
	minStatus.Gas = SystemStatus.Gas.TankPressure
	for elnum, el := range SystemStatus.Electrolysers {
		minEl := new(minElectrolyserStatus)
		if elnum == 0 {
			minEl.On = SystemStatus.Relays.EL0
		} else {
			minEl.On = SystemStatus.Relays.EL1
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
		IP                    string      `json:"ip"`
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

	type ACStatus struct {
		Power       jsonFloat32 `json:"watts"`
		Current     jsonFloat32 `json:"amps"`
		Voltage     jsonFloat32 `json:"volts"`
		Frequency   jsonFloat32 `json:"hertz"`
		PowerFactor jsonFloat32 `json:"powerfactor"`
		Energy      jsonFloat32 `json:"energy"`
	}

	type RelaysStatus struct {
		El0       bool `json:"el0"`
		El1       bool `json:"el1"`
		Gas       bool `json:"gas"`
		FC0Enable bool `json:"fc0en"`
		FC0Run    bool `json:"fc0run"`
		FC1Enable bool `json:"fc1en"`
		FC1Run    bool `json:"fc1run"`
		Spare     bool `json:"spare"`
	}

	var Status struct {
		Relays        RelaysStatus          `json:"relays"`
		Electrolysers []*ElectrolyserStatus `json:"el"`
		Dryer         DryerStatus           `json:"dr"`
		FuelCells     []*FuelCellStatus     `json:"fc"`
		Gas           GasStatus             `json:"gas"`
		Tds           float32               `json:"tds"`
		AC            ACStatus              `json:"ac"`
		HP            ACStatus              `json:"hp"`
	}
	Status.Gas.FuelCellPressure = jsonFloat32(math.Round(float64(SystemStatus.Gas.FuelCellPressure)*10) / 10)
	Status.Gas.TankPressure = jsonFloat32(math.Round(float64(SystemStatus.Gas.TankPressure)*10) / 10)
	Status.Relays.Gas = SystemStatus.Relays.GasToFuelCell
	Status.Relays.El0 = SystemStatus.Relays.EL0
	Status.Relays.El1 = SystemStatus.Relays.EL1
	Status.Relays.FC0Enable = SystemStatus.Relays.FC0Enable
	Status.Relays.FC0Run = SystemStatus.Relays.FC0Run
	Status.Relays.FC1Enable = SystemStatus.Relays.FC1Enable
	Status.Relays.FC1Run = SystemStatus.Relays.FC1Run
	Status.Relays.Spare = SystemStatus.Relays.Spare
	Status.Tds = SystemStatus.TDS.TdsReading
	Status.AC.Voltage = jsonFloat32(float32(SystemStatus.AC.ACVolts) / 100)
	Status.AC.Current = jsonFloat32(float32(SystemStatus.AC.ACCurrent) / 100)
	Status.AC.Power = jsonFloat32(float32(SystemStatus.AC.ACPower) / 100)
	Status.AC.Frequency = jsonFloat32(float32(SystemStatus.AC.ACFrequency) / 100)
	Status.AC.PowerFactor = jsonFloat32(float32(SystemStatus.AC.ACPowerFactor) / 100)
	Status.AC.Energy = jsonFloat32(float32(SystemStatus.AC.ACEnergy))
	Status.HP.Voltage = jsonFloat32(float32(SystemStatus.HP.ACVolts) / 100)
	Status.HP.Current = jsonFloat32(float32(SystemStatus.HP.ACCurrent) / 100)
	Status.HP.Power = jsonFloat32(float32(SystemStatus.HP.ACPower) / 100)
	Status.HP.Frequency = jsonFloat32(float32(SystemStatus.HP.ACFrequency) / 100)
	Status.HP.PowerFactor = jsonFloat32(float32(SystemStatus.HP.ACPowerFactor) / 100)
	Status.HP.Energy = jsonFloat32(float32(SystemStatus.HP.ACEnergy))
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
		ElStatus.IP = el.ip.String()
		if elnum == 0 {
			ElStatus.On = SystemStatus.Relays.EL0
		} else {
			ElStatus.On = SystemStatus.Relays.EL1
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
		FcStatus.InletTemp = jsonFloat32(math.Round(float64(fc.InletTemp)/10) / 10)
		FcStatus.OutletTemp = jsonFloat32(math.Round(float64(fc.OutletTemp)/10) / 10)
		FcStatus.Power = fc.OutputPower
		FcStatus.Amps = jsonFloat32(math.Round(float64(fc.OutputCurrent)/10) / 10)
		FcStatus.Volts = jsonFloat32(math.Round(float64(fc.OutputVolts)/10) / 10)
		FcStatus.State = fc.GetState()
		FcStatus.FaultA = strings.Join(getFuelCellError('A', fc.getFaultA()), ":")
		FcStatus.FaultB = strings.Join(getFuelCellError('B', fc.getFaultB()), ":")
		FcStatus.FaultC = strings.Join(getFuelCellError('C', fc.getFaultC()), ":")
		FcStatus.FaultD = strings.Join(getFuelCellError('D', fc.getFaultD()), ":")
		//		log.Println("Anode pressure = ", fc.AnodePressure)
		FcStatus.AnodePressure = jsonFloat32(float32(fc.AnodePressure) / 10)

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
	var body OnOffPayload

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
	if err := mbusRTU.GasOnOff(body.State); err != nil {
		ReturnJSONError(w, "Gas", err, http.StatusInternalServerError, true)
		return
	}
	returnJSONSuccess(w)
}

/**
Turn on the spare solenoid
*/
func setSpare(w http.ResponseWriter, r *http.Request) {

	var body OnOffPayload

	if bytes, err := io.ReadAll(r.Body); err != nil {
		ReturnJSONError(w, "Spare", err, http.StatusInternalServerError, true)
		return
	} else {
		debugPrint(string(bytes))
		if err := json.Unmarshal(bytes, &body); err != nil {
			ReturnJSONError(w, "Spare", err, http.StatusBadRequest, true)
			return
		}
	}
	if err := mbusRTU.SpareOnOff(body.State); err != nil {
		ReturnJSONError(w, "Spare", err, http.StatusInternalServerError, true)
		return
	}
	returnJSONSuccess(w)
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
	var System struct {
		Relays              *relayStatus
		AC                  *acStatus
		NumElectrolyser     uint8
		NumFuelCell         uint8
		FuelCellMaintenance bool
	}
	SystemStatus.m.Lock()
	defer SystemStatus.m.Unlock()
	mbusRTU.getRelayStatus()
	mbusRTU.getACStatus()

	System.Relays = &SystemStatus.Relays
	System.AC = &SystemStatus.AC
	var err error
	System.NumElectrolyser = uint8(len(SystemStatus.Electrolysers))
	System.NumFuelCell = uint8(len(canBus.fuelCell))
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
		select sum(greatest(ifnull(fc0OutputPower, 0) + ifnull(fc1OutputPower, 0), 0)) / 3600 as power
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

	Saved.Active, Saved.Since, err = calculateCO2Saved(`select ((sum(fc0OutputPower) + ifnull(sum(fc1OutputPower), 0)) / 3600000) * 0.16 as co2, min(logged) as since from logging`)
	if err != nil {
		ReturnJSONError(w, "CO2", err, http.StatusInternalServerError, true)
		return
	}
	Saved.Archive, Saved.Since, err = calculateCO2Saved(`select ((sum(fc0OutputPower) + ifnull(sum(fc1OutputPower), 0)) / 60000) * 0.16 as co2, min(logged) as since from logging_archive`)
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

func init() {
	var (
		CommsPort         string
		BaudRate          uint
		DataBits          uint
		StopBits          uint
		Parity            string
		TimeoutSecs       uint
		RelaySlaveAddress uint
		ACSlaveAddress    uint
		HPSlaveAddress    uint
	)
	SystemStatus.valid = false // Prevents logging until we have some actual data
	// Set up logging
	logwriter, e := syslog.New(syslog.LOG_NOTICE, "FireflyWeb")
	if e == nil {
		log.SetOutput(logwriter)
	}

	// Root password for database = 'ElektrikGreen2022'
	// Get the settings
	flag.StringVar(&databaseServer, "sqlServer", "localhost", "MySQL Server")
	flag.StringVar(&databaseName, "database", "firefly", "Database name")
	flag.StringVar(&databaseLogin, "dbUser", "FireflyService", "Database login user name")
	flag.StringVar(&databasePassword, "dbPassword", "logger", "Database user password")
	flag.StringVar(&databasePort, "dbPort", "3306", "Database port")
	flag.StringVar(&CANInterface, "can", "can0", "CAN Interface Name")
	flag.StringVar(&jsonSettings, "jsonSettings", "/etc/FireFlyWeb.json", "JSON file containing the system control parameters")

	// Modbus RTU stuff
	flag.StringVar(&CommsPort, "Port", "rtu:///dev/ttyUSB0", "communication port for the Modbus RTU equipment")
	flag.UintVar(&BaudRate, "baudrate", 9600, "communication port baud rate for the Modbus RTU equipment")
	flag.UintVar(&DataBits, "databits", 8, "communication port data bits for the Modbus RTU equipment")
	flag.UintVar(&StopBits, "stopbits", 1, "communication port stop bits for the Modbus RTU equipment")
	flag.StringVar(&Parity, "parity", "N", "communication port parity for the Modbus RTU equipment")
	flag.UintVar(&TimeoutSecs, "timeout", 5, "communication port timeout in seconds for the Modbus RTU equipment")
	flag.UintVar(&RelaySlaveAddress, "relayslave", 10, "Modbus slave ID for the Modbus RTU equipment handling the relays and analogue input")
	flag.UintVar(&ACSlaveAddress, "acslave", 1, "Modbus slave ID for the AC measurement device")
	flag.UintVar(&HPSlaveAddress, "hpslave", 20, "Modbus slave ID for the HeatPump AC measurement device")

	flag.Parse()
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	params = NewJsonSettings()
	if err := params.ReadSettings(jsonSettings); err != nil {
		log.Panic("Error reading the JSON settings file - ", err)
	}
	if params.DebugOutput {
		log.Println("running in debug mode")
	} else {
		log.Println("running in non-debug mode")
	}

	if dbPtr, err := connectToDatabase(); err != nil {
		log.Println(`Cannot connect to the database - `, err)
	} else {
		pDB = dbPtr
	}

	canBus = initCANLogger()
	mbusRTU = NewModbusRTUIO(CommsPort, BaudRate, DataBits, StopBits, Parity, TimeoutSecs, uint8(RelaySlaveAddress), uint8(ACSlaveAddress), uint8(HPSlaveAddress))

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

/*
AcquireFuelCells will turn on the fuel cells and wait 30 secnds then turn them off again
*/
func AcquireFuelCells() {
	for s := 0; s < 10; s++ {
		if mbusRTU.Active {
			break
		}
		time.Sleep(time.Second)
	}

	if !mbusRTU.Active {
		log.Fatal("Timed out waiting for Modbus Relays to come on line.")
	}

	if !mbusRTU.fc0en {
		if err := mbusRTU.FC0OnOff(true); err != nil {
			log.Print(err)
		}
		time.AfterFunc(time.Second*15, func() {
			if err := mbusRTU.FC0OnOff(false); err != nil {
				log.Print(err)
			}
		})
	}
	if !mbusRTU.fc1en {
		if err := mbusRTU.FC1OnOff(true); err != nil {
			log.Print(err)
		}
		time.AfterFunc(time.Second*15, func() {
			if err := mbusRTU.FC1OnOff(false); err != nil {
				log.Print(err)
			}
		})
	}
}

func main() {
	dataSignal = sync.NewCond(&sync.Mutex{})
	statusSignal = sync.NewCond(&sync.Mutex{})

	log.Println("Starting the CAN logger")
	go canBus.logCANData()
	log.Println("Starting the CAN monitor")
	go canBus.CanBusMonitor()
	log.Println("Starting the Modbus RTU manager")
	go mbusRTU.StartModbusIO()

	for _, el := range params.Electrolysers {
		if el.ID == 0 {
			IP := net.ParseIP(el.IP)
			electrolyser := NewElectrolyser(IP)
			SystemStatus.Electrolysers = append(SystemStatus.Electrolysers, electrolyser)
		}
	}
	if len(SystemStatus.Electrolysers) == 1 {
		for _, el := range params.Electrolysers {
			if el.ID == 1 {
				IP := net.ParseIP(el.IP)
				electrolyser := NewElectrolyser(IP)
				SystemStatus.Electrolysers = append(SystemStatus.Electrolysers, electrolyser)
			}
		}
	}

	if len(SystemStatus.Electrolysers) == 0 {
		go AcquireElectrolysers()
	}
	AcquireFuelCells()

	// Start the logging loop
	loggingLoop()
}
