package main

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/brutella/can"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// LoggerSQLStatement is the Logger SQL statement to save a 'CAN' frame
const LoggerSQLStatement = `INSERT INTO firefly.CAN_Data(id, canData, Event) VALUES(?,?,?)`

type Frame struct { // CAN bus frames from the FCM804
	id   uint32 // ID of the frame. The cell iD is in bits 0..2
	data uint64 // 8 bytes of data for each frame which we store in the database as single uint64
}

type CANBus struct {
	dataSet         [4096]Frame        // Ring buffer of frames in which to store incoling messages
	ringStart       int                // Pointer to the start of the buffer
	ringEnd         int                // pointer to the end of the buffer
	saving          bool               // Are we currently saving the buffer
	onDemandEnd     time.Time          // If this time is in the future we log all buffers of interest immediately to the database
	fuelCell        map[uint16]*FCM804 // A map of all the fuel cells in the system indexed byt their ID.
	lastLoggedEvent string             // Event date/time for the most recently logged event
	waitForLogger   bool               // If true, the current buffer will be saved when the next 0x400 frame with an ID of 0 is received
	ringCount       int                // Number of records in the ring buffer - We will not write log files with less than 960 entries
}

type CANFaultDefinition struct {
	flagLevel   uint8
	tag         string
	description string
	reboot      bool
}

var errorDefinitions map[uint16]CANFaultDefinition

func (pLogger *CANBus) loadFaultDefinitions() {
	var err error
	var key uint16
	var faultDescription CANFaultDefinition
	errorDefinitions = make(map[uint16]CANFaultDefinition, 128)

	if pDB == nil {
		pDB, err = connectToDatabase()
		if err != nil {
			log.Println(err)
			return
		}
	}
	rows, err := pDB.Query("SELECT (ascii(FaultType) + Flag) as `key`, Tag as `tag`, Description as `description`, Severity as `flagLevel`, Reboot as `reboot` FROM FcFaultDescriptions ORDER BY FaultType, Flag")
	if err != nil {
		log.Println(err)
		return
	}
	for rows.Next() {
		if err := rows.Scan(&key, &faultDescription.tag, &faultDescription.description, &faultDescription.flagLevel, &faultDescription.reboot); err != nil {
			log.Println(err)
			return
		}
		errorDefinitions[key] = faultDescription
	}
}

/**
getMaxFaultLevel scans the fault definition table and returns the highest severity for any fault in the four provided masks and if a reboot is needed to clear the fault
*/
func (pLogger *CANBus) getMaxFaultLevel(faultA uint32, faultB uint32, faultC uint32, faultD uint32) (maxLevel uint8, reboot bool) {
	if (faultA | faultB | faultC | faultD) == 0 {
		return 0, false
	}
	maxLevel = 0
	reboot = false
	mask := uint32(1)
	b := uint16(0)
	for b = 'A' * 256; b < ('A'*256)+32; b++ {
		if ((mask << b) & faultA) != 0 {
			if maxLevel < errorDefinitions[b].flagLevel {
				maxLevel = errorDefinitions[b].flagLevel
			}
			if errorDefinitions[b].reboot {
				reboot = true
			}
		}
	}
	for b = 'B' * 256; b < ('B'*256)+32; b++ {
		if ((mask << b) & faultB) != 0 {
			if maxLevel < errorDefinitions[b].flagLevel {
				maxLevel = errorDefinitions[b].flagLevel
			}
			if errorDefinitions[b].reboot {
				reboot = true
			}
		}
	}
	for b = 'C' * 256; b < ('C'*256)+32; b++ {
		if ((mask << b) & faultC) != 0 {
			if maxLevel < errorDefinitions[b].flagLevel {
				maxLevel = errorDefinitions[b].flagLevel
			}
			if errorDefinitions[b].reboot {
				reboot = true
			}
		}
	}
	for b = 'D' * 256; b < ('D'*256)+32; b++ {
		if ((mask << b) & faultD) != 0 {
			if maxLevel < errorDefinitions[b].flagLevel {
				maxLevel = errorDefinitions[b].flagLevel
			}
			if errorDefinitions[b].reboot {
				reboot = true
			}
		}
	}
	return maxLevel, reboot
}

// setOnDemandRecording will set the onDemandEnd date/time. Immediate logging of data will begin from this point until
// the designated end time. Only 0x400 frames are recorded but these can be extracted and sent to Intelligent energy for analysis.
func (pLogger *CANBus) setOnDemandRecording(until time.Time) {
	pLogger.onDemandEnd = until
}

// handleCANFrame figures out what to do with each CAN frame received
func (pLogger *CANBus) handleCANFrame(frm can.Frame) {
	var data uint64
	var event sql.NullTime
	device := uint16(frm.ID & 7)
	frameID := frm.ID & 0xFFFFFFF8
	if (frameID != 0x400) && (frameID != 0x6f0) {
		fcm, found := pLogger.fuelCell[device]
		if !found {
			fmt.Printf("Adding fuel cell %d - Frame with %04x\n", device, frameID)
			// We don't have this device in our map, so we should add it.
			pLogger.fuelCell[device] = NewFCM804(pLogger, device)
			fcm = pLogger.fuelCell[device]
		}
		if fcm.ProcessFrame(frameID, frm.Data[:]) {
			// We got a fault condition change, so we should wait until the current 0x400 frame sequence completes then log the buffer
			pLogger.waitForLogger = true
			if debug {
				log.Println("Error found - Waiting for a full frame to recodrd the data.")
			}
		}
		// We only record 0x40x frames
		return
	}
	if frameID != 0x400 {
		// Ignore developer frames
		return
	}
	// Set the last update time for the fuel cell
	pLogger.fuelCell[device].LastUpdate = time.Now()
	data = binary.BigEndian.Uint64(frm.Data[:])
	// Don't mess with the buffer if we are writing it to the database
	if !pLogger.saving {
		if pLogger.onDemandEnd.Before(time.Now()) {
			// We are past the onDemandEnd time so not continously logging to the database
			pLogger.dataSet[pLogger.ringEnd].id = frm.ID
			pLogger.dataSet[pLogger.ringEnd].data = data
			pLogger.ringEnd++
			if pLogger.ringEnd >= len(pLogger.dataSet) {
				pLogger.ringEnd = 0
			}
			pLogger.ringCount++
			if pLogger.ringCount > len(pLogger.dataSet) {
				pLogger.ringCount = len(pLogger.dataSet)
			}
			if pLogger.ringEnd <= pLogger.ringStart {
				pLogger.ringStart++
				if pLogger.ringStart >= len(pLogger.dataSet) {
					pLogger.ringStart = 0
				}
			}
			if frm.Data[0] == 0x2E && pLogger.waitForLogger && pLogger.ringCount > 960 {
				pLogger.logCanFrames()
				//} else {
				//	if debug && pLogger.waitForLogger {
				//		log.Println(frm.Data[0], pLogger.ringCount)
				//	}
			}
		} else {
			// We are doing an on demand recording here
			// Reset the ring buffer
			pLogger.ringStart = 0
			pLogger.ringEnd = 0
			// Log the current frame to the database
			if pDB != nil {
				event.Valid = false // Set the event false if we are on demand logging
				_, err := pDB.Exec(LoggerSQLStatement, frm.ID, data, event)
				if err != nil {
					log.Println(err)
					if err := pDB.Close(); err != nil {
						log.Println(err)
					}
					pDB = nil
					pLogger.ringStart = 0
					pLogger.ringEnd = 0
					pLogger.saving = false
					pLogger.onDemandEnd = time.Now()
					return
				}
			}
		}
	}
}

// logCanFrame logs the current ring buffer contents to the database
func (pLogger *CANBus) logCanFrames() {
	// When we leave here we need to have reset the ring buffer
	defer func() {
		pLogger.ringStart = 0
		pLogger.ringEnd = 0
		pLogger.ringCount = 0
		pLogger.saving = false
		pLogger.waitForLogger = false
	}()
	if debug {
		log.Println("Log CAN data")
	}
	var err error
	var event sql.NullTime
	event.Time = time.Now()
	event.Valid = true
	pLogger.saving = true
	if pDB == nil {
		pDB, err = connectToDatabase()
		if err != nil {
			log.Println(err)
			return
		}
	}
	pLogger.lastLoggedEvent = event.Time.Format("2006-01-02T15:04:05:06")
	if debug {
		log.Println("Logging event - ", pLogger.lastLoggedEvent)
	}

	for {
		_, err := pDB.Exec(LoggerSQLStatement, pLogger.dataSet[pLogger.ringStart].id, pLogger.dataSet[pLogger.ringStart].data, event)
		if err != nil {
			log.Println(err)
			if err := pDB.Close(); err != nil {
				log.Println(err)
			}
			pDB = nil
			return
		}
		pLogger.ringStart++
		if pLogger.ringStart >= len(pLogger.dataSet) {
			pLogger.ringStart = 0
		}
		if pLogger.ringStart == pLogger.ringEnd {
			return
		}
	}
}

// CanBusMonitor starts the CAN bus monitor and logger
func (pLogger *CANBus) CanBusMonitor() {

	for {
		bus, err := can.NewBusForInterfaceWithName("can0")
		if err != nil {
			log.Println("CAN interface not available.", err)
			//		log.Panicf("Error starting CAN interface - %s -\nSorry, I am giving up", err)
		} else {
			bus.SubscribeFunc(pLogger.handleCANFrame)
			err = bus.ConnectAndPublish()
			if err != nil {
				log.Println("ConnectAndPublish failed, cannot log CAN frames.", err)
			}
		}
		// If something goes wrong sleep for 10 seconds and try again.
		time.Sleep(time.Second * 10)
	}

}

/**
Set up the logger and start processing 'CAN' frames
*/
func initCANLogger(numFuelCells uint16) *CANBus {
	logger := new(CANBus)
	logger.ringStart = 0
	logger.ringEnd = 0
	logger.saving = false
	logger.fuelCell = make(map[uint16]*FCM804)
	for cell := uint16(0); cell < numFuelCells; cell++ {
		logger.fuelCell[cell] = NewFCM804(logger, cell)
	}
	logger.onDemandEnd = time.Now()

	go logger.CanBusMonitor()

	return logger
}

func ReturnCanDumpResult(w http.ResponseWriter, rows *sql.Rows, file *os.File) {
	var jErr JSONError
	var oneRow struct {
		logged float64
		ID     uint16
		Data   uint64
	}
	var filename = file.Name()

	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err)
		}
	}()
	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err)
		}
	}()
	rownum := 0
	start := 0.0
	for rows.Next() {
		err := rows.Scan(&oneRow.logged, &oneRow.ID, &oneRow.Data)
		if err != nil {
			jErr.AddError("candump", err)
			jErr.ReturnError(w, 500)
			return
		}
		if start == 0.0 {
			start = oneRow.logged
			epoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
			t := time.Unix(int64(start), 0)
			excelTime := float64(t.Sub(epoch)) / float64(time.Hour*24)

			if _, err := fmt.Fprintf(file, `;$FILEVERSION=1.35.255.0
;$STARTTIME=%f
;
;   %s
;   Start time:%s
;   Generated by FireflyWeb:
;-------------------------------------------------------------------------------
;   Bus Name Connection              Protocol  Bit rate
;   2   USB CAN                      N/A       N/A     
;-------------------------------------------------------------------------------
;   Message Number
;   |         Time Offset(ms)
;   |         |       Bus
;   |         |       |    Type
;   |         |       |    |       ID(hex)
;   |         |       |    |       |    Reserved
;   |         |       |    |       |    |   Data Length Code
;   |         |       |    |       |    |   |    Data Bytes(hex) ...
;   |         |       |    |       |    |   |    |
;   |         |       |    |       |    |   |    |
;   |         |       |    |       |    |   |    |
;---+-- ------+------ +- --+-- ----+--- +- -+-- -+ -- -- -- -- -- -- --
`,
				excelTime, file.Name(), t.Format("01/02/2006 03:04:05 PM")); err != nil {
				jErr.AddError("candump", err)
				jErr.ReturnError(w, 500)
				return
			}
		}
		oneRow.logged = oneRow.logged - start
		buf := new(bytes.Buffer)
		err = binary.Write(buf, binary.BigEndian, oneRow.Data)
		if err != nil {
			jErr.AddError("candump", err)
			jErr.ReturnError(w, 500)
			return
		}
		if _, err = fmt.Fprintf(file, "%6d)%13.3f  2  Rx        %04X -  8    % X\n",
			rownum, oneRow.logged, oneRow.ID, buf); err != nil {
			log.Println(err)
			return
		}
		rownum++
	}
	if err := file.Close(); err != nil {
		jErr.AddError("candump", err)
		jErr.ReturnError(w, 500)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	file, err := os.Open(filename)
	if err != nil {
		jErr.AddError("candump", err)
		jErr.ReturnError(w, 500)
		return
	}
	if _, err := io.Copy(w, file); err != nil {
		jErr.AddError("candump", err)
		jErr.ReturnError(w, 500)
		return
	}
	if err := file.Close(); err != nil {
		log.Println(err)
	} else {
		if err := os.Remove(filename); err != nil {
			log.Println(err)
		}
	}
}

func candumpEvent(w http.ResponseWriter, r *http.Request) {
	var jErr JSONError
	var err error
	var stmt *sql.Stmt
	vars := mux.Vars(r)
	event := vars["event"]

	if stmt, err = pDB.Prepare(CANDUMPEVENTSQL); err != nil {
		jErr.AddError("candump", err)
		jErr.ReturnError(w, 500)
		return
	}

	rows, err := stmt.Query(event)
	if err != nil {
		jErr.AddError("candump", err)
		jErr.ReturnError(w, 500)
		return
	}

	now := time.Now()
	year, month, day := now.Date()
	filename := fmt.Sprintf("./log-%d-%02d-%02d-%02d-%02d.trc", year, month, day, now.Hour(), now.Minute())
	file, err := os.Create(filename)
	if err != nil {
		log.Println("Error creating can dump file [", filename, "] - ", err)
		jErr.AddError("candump", err)
		jErr.ReturnError(w, 500)
		return
	}

	ReturnCanDumpResult(w, rows, file)
}

func candump(w http.ResponseWriter, r *http.Request) {
	var jErr JSONError
	var err error
	var stmt *sql.Stmt
	vars := mux.Vars(r)
	from := vars["from"]
	to := vars["to"]

	if stmt, err = pDB.Prepare(CANDUMPSQL); err != nil {
		jErr.AddError("candump", err)
		jErr.ReturnError(w, 500)
		return
	}
	now := time.Now()
	year, month, day := now.Date()
	filename := fmt.Sprintf("./log-%d-%02d-%02d-%02d-%02d.trc", year, month, day, now.Hour(), now.Minute())
	file, err := os.Create(filename)
	if err != nil {
		log.Println("Error creating can dump file [", filename, "] - ", err)
		jErr.AddError("candump", err)
		jErr.ReturnError(w, 500)
		return
	}

	rows, err := stmt.Query(from, to)
	if err != nil {
		jErr.AddError("candump", err)
		jErr.ReturnError(w, 500)
		return
	}
	ReturnCanDumpResult(w, rows, file)
}

func canRecord(w http.ResponseWriter, r *http.Request) {
	var jErr JSONError
	vars := mux.Vars(r)
	to := vars["to"]

	toTime, err := time.ParseInLocation("2006-1-2T15:4:5", to, time.Local)
	if err != nil {
		jErr.AddError("candump", err)
		jErr.ReturnError(w, 400)
		return
	}
	n := time.Now()
	if !toTime.After(n) {
		log.Println("Bad time specified")
		errString := fmt.Sprintf("End of recording (%v) is in the past. (%v)", toTime, n)
		jErr.AddErrorString("canRecord", errString)
		jErr.ReturnError(w, 400)
		return
	}
	if toTime.Sub(time.Now()) > time.Hour*2 {
		jErr.AddErrorString("canRecord", "You can only ask for up to 4 hours of on demand CAN logging.")
		jErr.ReturnError(w, 400)
		return
	}
	canBus.setOnDemandRecording(toTime)
	_, err = fmt.Fprintf(w, `<html>
	<head>
		<title>Can Recording Started</title>
	</head>
	<body>
		<h1>Now recording CAN data until %s<h2><br />
		<a href="/">Back to menu</a>
	</body>
</html>`, to)
}

/**
listCANEvents returns a list of the last 50 CAN events
*/
func listCANEvents(w http.ResponseWriter, _ *http.Request) {
	var events struct {
		Events []*string `json:"event"`
	}
	var jErr JSONError
	rows, err := pDB.Query(LISTCANEVENTSSQL)
	if err != nil {
		jErr.AddError("candump", err)
		jErr.ReturnError(w, 500)
		return
	}

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
		row := new(string)
		if err := rows.Scan(&row); err != nil {
			log.Print(err)
		} else {
			events.Events = append(events.Events, row)
		}
	}
	if JSON, err := json.Marshal(events); err != nil {
		if _, err := fmt.Fprintf(w, `{"error":"%s"`, err.Error()); err != nil {
			log.Println("Error returning event list - ", err)
		}
	} else {
		if _, err := fmt.Fprintf(w, string(JSON)); err != nil {
			log.Println("Error returning event list - ", err)
		}
	}
}
