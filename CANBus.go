package main

import (
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
//const LoggerSQLStatement = `INSERT INTO firefly.CAN_Trace(id, canData, Event, OnDemand) VALUES(?,?,?,?)`
const LoggerSQLStatement = `INSERT INTO firefly.CAN_Trace
	(Cell, data00, data00offsetms, data01, data01offsetms, data02, data02offsetms, data03, data03offsetms,
	       data04, data04offsetms, data05, data05offsetms, data06, data06offsetms, data07, data07offsetms,
	       data08, data08offsetms, data09, data09offsetms, data0A, data0Aoffsetms, data0B, data0Boffsetms,
	       data0C, data0Coffsetms, data0D, data0Doffsetms, data0E, data0Eoffsetms, data0F, data0Foffsetms,
	       data10, data10offsetms, data11, data11offsetms, data12, data12offsetms, data13, data13offsetms,
	       data14, data14offsetms, data15, data15offsetms, data16, data16offsetms, data17, data17offsetms,
	       data18, data18offsetms, data19, data19offsetms, data1A, data1Aoffsetms, data1B, data1Boffsetms,
	       data1C, data1Coffsetms, data1D, data1Doffsetms, data1E, data1Eoffsetms, data1F, data1Foffsetms,
	       data20, data20offsetms, data21, data21offsetms, data22, data22offsetms, data23, data23offsetms,
	       data24, data24offsetms, data25, data25offsetms, data26, data26offsetms, data27, data27offsetms,
	       data28, data28offsetms, data29, data29offsetms, data2A, data2Aoffsetms, data2B, data2Boffsetms,
	       data2C, data2Coffsetms, data2D, data2Doffsetms, data2E, data2Eoffsetms,
	       Event, OnDemand)
   VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, 
             ?, ?, ?, ?, ?, ?, ?, ?, 
             ?, ?, ?, ?, ?, ?, ?, ?, 
             ?, ?, ?, ?, ?, ?, ?, ?, 
             ?, ?, ?, ?, ?, ?, ?, ?, 
             ?, ?, ?, ?, ?, ?, ?, ?, 
             ?, ?, ?, ?, ?, ?, ?, ?, 
             ?, ?, ?, ?, ?, ?, ?, ?, 
             ?, ?, ?, ?, ?, ?, ?, ?, 
             ?, ?, ?, ?, ?, ?, ?, ?, 
             ?, ?, ?, ?, ?, ?, ?, ?, 
             ?, ?, ?, ?, ?, ?, 
             ?, ?);
;`

type Frame0x400Data struct {
	Time      time.Time
	Cell      int8
	FrameData [47]struct {
		tOffset int16
		data    []byte
	}
}

var RecordingFrame *Frame0x400Data

var CanLogChannel chan *Frame0x400Data

type Frame struct { // CAN bus frames from the FCM804
	id   uint32 // ID of the frame. The cell iD is in bits 0..2
	data uint64 // 8 bytes of data for each frame which we store in the database as single uint64
}

type CANBus struct {
	EventTime       time.Time         // The time for the event we are recording
	OnDemand        bool              // True if we are logging on demand or false if because we encountered a fault
	dataSet         [4096]Frame       // Ring buffer of frames in which to store incoling messages
	ringStart       int               // Pointer to the start of the buffer
	ringEnd         int               // pointer to the end of the buffer
	saving          bool              // Are we currently saving the buffer
	onDemandEnd     time.Time         // If this time is in the future we log all buffers of interest immediately to the database
	fuelCell        map[uint8]*FCM804 // A map of all the fuel cells in the system indexed byt their ID.
	lastLoggedEvent string            // Event date/time for the most recently logged event
	waitForLogger   bool              // If true, the current buffer will be saved when the next 0x400 frame with an ID of 0 is received
	ringCount       int               // Number of records in the ring buffer - We will not write log files with less than 960 entries
	LogStatement    *sql.Stmt         // Prepared statement to log CAN frames
}

type CANFaultDefinition struct {
	flagLevel   uint8
	tag         string
	description string
	reboot      bool
}

var errorDefinitions map[uint16]CANFaultDefinition

var clearBufferTimer *time.Timer

func init() {
	CanLogChannel = make(chan *Frame0x400Data)
}

func (pLogger *CANBus) loadFaultDefinitions() {
	var err error
	var key uint16
	var faultDescription CANFaultDefinition
	errorDefinitions = make(map[uint16]CANFaultDefinition, 128)

	if pDB == nil {
		err = pLogger.ConnectToDatabase()
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

func (pLogger *CANBus) setEventDateTime() {
	if pLogger.EventTime == *new(time.Time) {
		pLogger.EventTime = time.Now()
	}
}

func (pLogger *CANBus) clearEventDateTime() {
	pLogger.EventTime = *new(time.Time)
}

// setOnDemandRecording will set the onDemandEnd date/time. Immediate logging of data will begin from this point until
// the designated end time. Only 0x400 frames are recorded but these can be extracted and sent to Intelligent energy for analysis.
func (pLogger *CANBus) setOnDemandRecording(until time.Time) {
	//	pLogger.setEventDateTime()
	pLogger.onDemandEnd = until
	pLogger.setEventDateTime()
	pLogger.OnDemand = true
}

func (pLogger *CANBus) clearBuffers() {
	for _, fc := range pLogger.fuelCell {
		debugPrint("Clear fuel cell %d", fc.device)
		fc.Clear()
	}
}

var next0x400id uint64

func (pLogger *CANBus) logCANData() {
	for {
		frame := <-CanLogChannel

		if time.Now().Before(pLogger.onDemandEnd) {
			pLogger.OnDemand = true
			pLogger.setEventDateTime()
		}
		if (params.FuelCellLogOnEnable && SystemStatus.Relays.FC0Enable) ||
			(params.FuelCellLogOnRun && SystemStatus.Relays.FC0Run) {
			pLogger.OnDemand = true
			pLogger.setEventDateTime()
		}

		if pLogger.OnDemand {
			if (pDB == nil) || (pLogger.LogStatement == nil) {
				log.Print("Database is not connected or log statment is closed in CAN logger")
				var err error
				err = pLogger.ConnectToDatabase()
				if err != nil {
					log.Print("Failed to connect ot the database - ", err)
				}
			}
			if pDB != nil {
				_, err := pLogger.LogStatement.Exec(frame.Cell,
					frame.FrameData[0].data[:], frame.FrameData[0].tOffset, frame.FrameData[1].data[:], frame.FrameData[1].tOffset, frame.FrameData[2].data[:], frame.FrameData[2].tOffset, frame.FrameData[3].data[:], frame.FrameData[3].tOffset,
					frame.FrameData[4].data[:], frame.FrameData[4].tOffset, frame.FrameData[5].data[:], frame.FrameData[5].tOffset, frame.FrameData[6].data[:], frame.FrameData[6].tOffset, frame.FrameData[7].data[:], frame.FrameData[7].tOffset,
					frame.FrameData[8].data[:], frame.FrameData[8].tOffset, frame.FrameData[9].data[:], frame.FrameData[9].tOffset, frame.FrameData[10].data[:], frame.FrameData[11].tOffset, frame.FrameData[11].data[:], frame.FrameData[11].tOffset,
					frame.FrameData[12].data[:], frame.FrameData[12].tOffset, frame.FrameData[13].data[:], frame.FrameData[13].tOffset, frame.FrameData[14].data[:], frame.FrameData[14].tOffset, frame.FrameData[15].data[:], frame.FrameData[15].tOffset,
					frame.FrameData[16].data[:], frame.FrameData[16].tOffset, frame.FrameData[17].data[:], frame.FrameData[17].tOffset, frame.FrameData[18].data[:], frame.FrameData[18].tOffset, frame.FrameData[19].data[:], frame.FrameData[19].tOffset,
					frame.FrameData[20].data[:], frame.FrameData[20].tOffset, frame.FrameData[21].data[:], frame.FrameData[21].tOffset, frame.FrameData[22].data[:], frame.FrameData[22].tOffset, frame.FrameData[23].data[:], frame.FrameData[23].tOffset,
					frame.FrameData[24].data[:], frame.FrameData[24].tOffset, frame.FrameData[25].data[:], frame.FrameData[25].tOffset, frame.FrameData[26].data[:], frame.FrameData[26].tOffset, frame.FrameData[27].data[:], frame.FrameData[27].tOffset,
					frame.FrameData[28].data[:], frame.FrameData[28].tOffset, frame.FrameData[29].data[:], frame.FrameData[29].tOffset, frame.FrameData[30].data[:], frame.FrameData[30].tOffset, frame.FrameData[31].data[:], frame.FrameData[31].tOffset,
					frame.FrameData[32].data[:], frame.FrameData[32].tOffset, frame.FrameData[33].data[:], frame.FrameData[33].tOffset, frame.FrameData[34].data[:], frame.FrameData[34].tOffset, frame.FrameData[35].data[:], frame.FrameData[35].tOffset,
					frame.FrameData[36].data[:], frame.FrameData[36].tOffset, frame.FrameData[37].data[:], frame.FrameData[37].tOffset, frame.FrameData[38].data[:], frame.FrameData[38].tOffset, frame.FrameData[39].data[:], frame.FrameData[39].tOffset,
					frame.FrameData[40].data[:], frame.FrameData[40].tOffset, frame.FrameData[41].data[:], frame.FrameData[41].tOffset, frame.FrameData[42].data[:], frame.FrameData[42].tOffset, frame.FrameData[43].data[:], frame.FrameData[43].tOffset,
					frame.FrameData[44].data[:], frame.FrameData[44].tOffset, frame.FrameData[45].data[:], frame.FrameData[45].tOffset, frame.FrameData[46].data[:], frame.FrameData[46].tOffset, pLogger.EventTime, pLogger.OnDemand)
				if err != nil {
					log.Println("CAN Bus log to database error", err)
					if err := pDB.Close(); err != nil {
						log.Println(err)
					}
					pDB = nil
				}
			} else {
				log.Println("Missed logging a CAN frame because of a database error")
			}
		}
	}
}

// handleCANFrame figures out what to do with each CAN frame received
func (pLogger *CANBus) handleCANFrame(frm can.Frame) {
	var data uint64

	// Ignore everything on the CAN bus during fuel cell maintenance
	if params.FuelCellMaintenance {
		return
	}

	//	Reset the timer
	if clearBufferTimer == nil || !clearBufferTimer.Reset(time.Second*5) {
		clearBufferTimer = time.AfterFunc(time.Second*5, pLogger.clearBuffers)
	}

	device := uint8(frm.ID & 7)
	frameID := frm.ID & 0xFFFFFFF8
	if (frameID != 0x400) && (frameID != 0x6f0) {
		fcm, found := pLogger.fuelCell[device]
		if !found {
			log.Printf("Adding fuel cell %d - Frame with %04x\n", device, frameID)
			// We don't have this device in our map, so we should add it.
			pLogger.fuelCell[device] = NewFCM804(pLogger, device)
			fcm = pLogger.fuelCell[device]
		}
		fcm.LastUpdate = time.Now()
		if fcm.ProcessFrame(frameID, frm.Data[:]) {
			// We got a fault condition change, so we should wait until the current 0x400 frame sequence completes then log the buffer
			pLogger.waitForLogger = true
			debugPrint("Error found - Waiting for a full frame to record the data.")
		}
		// We only record 0x40x frames
		return
	}
	if frameID != 0x400 {
		// Ignore developer frames
		return
	}
	// Set the last update time for the fuel cell
	if len(pLogger.fuelCell) <= int(device) {
		return
	}
	data = binary.BigEndian.Uint64(frm.Data[:])
	id := data >> 56
	if id != next0x400id {
		log.Printf("CAN log expected id = %02x but got %02x", next0x400id, id)
	}
	id = id + 1
	if id > 0x2E {
		id = 0
	}
	next0x400id = id

	if RecordingFrame == nil {
		RecordingFrame = new(Frame0x400Data)
		RecordingFrame.Time = time.Now()
	}
	RecordingFrame.Cell = 0
	FrameNumber := frm.Data[0]
	if FrameNumber > 0x2E {
		log.Println("Bad 0x400 frame ID = ", FrameNumber)
	} else {
		RecordingFrame.FrameData[FrameNumber].data = make([]byte, 7)
		RecordingFrame.FrameData[FrameNumber].tOffset = int16(time.Now().Sub(RecordingFrame.Time).Milliseconds())
		for idx, d := range frm.Data[1:8] {
			RecordingFrame.FrameData[FrameNumber].data[idx] = d
		}
	}
	if FrameNumber == 0x2E {
		// Last frame so switch to allow saving the completed set to the database
		CanLogChannel <- RecordingFrame
		RecordingFrame = nil
	}
}

//// logCanFrame logs the current ring buffer contents to the database
//func (pLogger *CANBus) logCanFrames() {
//	// When we leave here we need to have reset the ring buffer
//	defer func() {
//		pLogger.ringStart = 0
//		pLogger.ringEnd = 0
//		pLogger.ringCount = 0
//		pLogger.saving = false
//		pLogger.waitForLogger = false
//	}()
//	if params.DebugOutput {
//		log.Println("Log CAN data")
//	}
//	var err error
//	var event sql.NullTime
//	event.Time = time.Now()
//	event.Valid = true
//	pLogger.saving = true
//	if pDB == nil {
//		pDB, err = connectToDatabase()
//		if err != nil {
//			log.Println(err)
//			return
//		}
//	}
//	pLogger.lastLoggedEvent = event.Time.Format("2006-01-02T15:04:05:06")
//	if params.DebugOutput {
//		log.Println("Logging event - ", pLogger.lastLoggedEvent)
//	}
//
//	for {
//		_, err := pDB.Exec(LoggerSQLStatement, pLogger.dataSet[pLogger.ringStart].id, pLogger.dataSet[pLogger.ringStart].data, event, false)
//		if err != nil {
//			log.Println(err)
//			if err := pDB.Close(); err != nil {
//				log.Println(err)
//			}
//			pDB = nil
//			return
//		}
//		pLogger.ringStart++
//		if pLogger.ringStart >= len(pLogger.dataSet) {
//			pLogger.ringStart = 0
//		}
//		if pLogger.ringStart == pLogger.ringEnd {
//			return
//		}
//	}
//}

/*
ConnectToDatabase will reconnect to the database if it is disconnected and prepare the logging statement
*/
func (pLogger *CANBus) ConnectToDatabase() error {
	if pDB == nil {
		if dbConn, err := connectToDatabase(); err != nil {
			return err
		} else {
			pDB = dbConn
		}
	}
	if stmt, err := pDB.Prepare(LoggerSQLStatement); err != nil {
		log.Print("Error preparing CAN logger statement - ", err)
		if err := pDB.Close(); err != nil {
			log.Println(err)
		}
		pDB = nil
		return err
	} else {
		pLogger.LogStatement = stmt
	}
	return nil
}

// CanBusMonitor starts the CAN bus monitor and logger
func (pLogger *CANBus) CanBusMonitor() {
	for {

		bus, err := can.NewBusForInterfaceWithName("can0")
		if err != nil {
			log.Println("CAN interface not available.", err)
		} else {
			if pDB == nil {
				if err = pLogger.ConnectToDatabase(); err != nil {
					log.Println(err)
					return
				}
			}
			log.Println("Subscribing the handleCANFrame function")
			bus.SubscribeFunc(pLogger.handleCANFrame)
			err = bus.ConnectAndPublish()
			if err != nil {
				log.Println("ConnectAndPublish failed, cannot log CAN frames.", err)
			} else {
				log.Println("Logging CAN data from the fuel cells from can0")
			}
		}
		// If something goes wrong sleep for 10 seconds and try again.
		time.Sleep(time.Second * 10)
	}
}

/**
Set up the logger and start processing 'CAN' frames
*/
func initCANLogger() *CANBus {
	logger := new(CANBus)
	logger.ringStart = 0
	logger.ringEnd = 0
	logger.saving = false
	logger.fuelCell = make(map[uint8]*FCM804)
	logger.onDemandEnd = time.Now()

	return logger
}

func ReturnCanDumpResult(w http.ResponseWriter, rows *sql.Rows, file *os.File) {
	var oneRow struct {
		logged float64
		Data   Frame0x400Data
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
	//	var frame [47][]byte
	for rows.Next() {
		err := rows.Scan(&oneRow.logged, &oneRow.Data.Cell,
			&oneRow.Data.FrameData[0].data, &oneRow.Data.FrameData[0].tOffset, &oneRow.Data.FrameData[1].data, &oneRow.Data.FrameData[1].tOffset, &oneRow.Data.FrameData[2].data, &oneRow.Data.FrameData[2].tOffset, &oneRow.Data.FrameData[3].data, &oneRow.Data.FrameData[3].tOffset,
			&oneRow.Data.FrameData[4].data, &oneRow.Data.FrameData[4].tOffset, &oneRow.Data.FrameData[5].data, &oneRow.Data.FrameData[5].tOffset, &oneRow.Data.FrameData[6].data, &oneRow.Data.FrameData[6].tOffset, &oneRow.Data.FrameData[7].data, &oneRow.Data.FrameData[7].tOffset,
			&oneRow.Data.FrameData[8].data, &oneRow.Data.FrameData[8].tOffset, &oneRow.Data.FrameData[9].data, &oneRow.Data.FrameData[9].tOffset, &oneRow.Data.FrameData[10].data, &oneRow.Data.FrameData[10].tOffset, &oneRow.Data.FrameData[11].data, &oneRow.Data.FrameData[11].tOffset,
			&oneRow.Data.FrameData[12].data, &oneRow.Data.FrameData[12].tOffset, &oneRow.Data.FrameData[13].data, &oneRow.Data.FrameData[13].tOffset, &oneRow.Data.FrameData[14].data, &oneRow.Data.FrameData[14].tOffset, &oneRow.Data.FrameData[15].data, &oneRow.Data.FrameData[15].tOffset,
			&oneRow.Data.FrameData[16].data, &oneRow.Data.FrameData[16].tOffset, &oneRow.Data.FrameData[17].data, &oneRow.Data.FrameData[17].tOffset, &oneRow.Data.FrameData[18].data, &oneRow.Data.FrameData[18].tOffset, &oneRow.Data.FrameData[19].data, &oneRow.Data.FrameData[19].tOffset,
			&oneRow.Data.FrameData[20].data, &oneRow.Data.FrameData[20].tOffset, &oneRow.Data.FrameData[21].data, &oneRow.Data.FrameData[21].tOffset, &oneRow.Data.FrameData[22].data, &oneRow.Data.FrameData[22].tOffset, &oneRow.Data.FrameData[23].data, &oneRow.Data.FrameData[23].tOffset,
			&oneRow.Data.FrameData[24].data, &oneRow.Data.FrameData[24].tOffset, &oneRow.Data.FrameData[25].data, &oneRow.Data.FrameData[25].tOffset, &oneRow.Data.FrameData[26].data, &oneRow.Data.FrameData[26].tOffset, &oneRow.Data.FrameData[27].data, &oneRow.Data.FrameData[27].tOffset,
			&oneRow.Data.FrameData[28].data, &oneRow.Data.FrameData[28].tOffset, &oneRow.Data.FrameData[29].data, &oneRow.Data.FrameData[29].tOffset, &oneRow.Data.FrameData[30].data, &oneRow.Data.FrameData[30].tOffset, &oneRow.Data.FrameData[31].data, &oneRow.Data.FrameData[31].tOffset,
			&oneRow.Data.FrameData[32].data, &oneRow.Data.FrameData[32].tOffset, &oneRow.Data.FrameData[33].data, &oneRow.Data.FrameData[33].tOffset, &oneRow.Data.FrameData[34].data, &oneRow.Data.FrameData[34].tOffset, &oneRow.Data.FrameData[35].data, &oneRow.Data.FrameData[35].tOffset,
			&oneRow.Data.FrameData[36].data, &oneRow.Data.FrameData[36].tOffset, &oneRow.Data.FrameData[37].data, &oneRow.Data.FrameData[37].tOffset, &oneRow.Data.FrameData[38].data, &oneRow.Data.FrameData[38].tOffset, &oneRow.Data.FrameData[39].data, &oneRow.Data.FrameData[39].tOffset,
			&oneRow.Data.FrameData[40].data, &oneRow.Data.FrameData[40].tOffset, &oneRow.Data.FrameData[41].data, &oneRow.Data.FrameData[41].tOffset, &oneRow.Data.FrameData[42].data, &oneRow.Data.FrameData[42].tOffset, &oneRow.Data.FrameData[43].data, &oneRow.Data.FrameData[43].tOffset,
			&oneRow.Data.FrameData[44].data, &oneRow.Data.FrameData[44].tOffset, &oneRow.Data.FrameData[45].data, &oneRow.Data.FrameData[45].tOffset, &oneRow.Data.FrameData[46].data, &oneRow.Data.FrameData[46].tOffset)
		if err != nil {
			ReturnJSONError(w, "canDump", err, 500, true)
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
				ReturnJSONError(w, "canDump", err, 500, true)
				return
			}
		}
		oneRow.logged = oneRow.logged - start
		var byteBuffer [8]byte
		//		err = binary.Write(buf, binary.BigEndian, oneRow.Data)
		//		if err != nil {
		//			ReturnJSONError(w, "canDump", err, 500, true)
		//			return
		//		}

		for id, frm := range oneRow.Data.FrameData {
			byteBuffer[0] = byte(id)
			for i, b := range frm.data {
				byteBuffer[i+1] = b
			}
			if _, err = fmt.Fprintf(file, "%6d)%13d  2  Rx        %04X -  8    % X\n",
				rownum, (int64(oneRow.logged)*1000)+int64(frm.tOffset), 0x400+int(oneRow.Data.Cell), byteBuffer); err != nil {
				log.Println(err)
				return
			}
			rownum++
		}
	}
	if err := file.Close(); err != nil {
		ReturnJSONError(w, "canDump", err, 500, true)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	file, err := os.Open(filename)
	if err != nil {
		ReturnJSONError(w, "canDump", err, 500, true)
		return
	}
	if _, err := io.Copy(w, file); err != nil {
		ReturnJSONError(w, "canDump", err, 500, true)
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
		ReturnJSONError(w, "candump", err, http.StatusInternalServerError, true)
		return
	}

	rows, err := stmt.Query(event)
	if err != nil {
		ReturnJSONError(w, "candump", err, http.StatusInternalServerError, true)
		jErr.ReturnError(w, 500)
		return
	}

	now := time.Now()
	year, month, day := now.Date()
	filename := fmt.Sprintf("./log-%d-%02d-%02d-%02d-%02d.trc", year, month, day, now.Hour(), now.Minute())
	file, err := os.Create(filename)
	if err != nil {
		log.Println("Error creating can dump file [", filename, "] - ", err)
		err := jErr.AddError("candump", err)
		if err != nil {
			return
		}
		jErr.ReturnError(w, 500)
		return
	}

	ReturnCanDumpResult(w, rows, file)
}

func candump(w http.ResponseWriter, r *http.Request) {
	var err error
	var stmt *sql.Stmt
	vars := mux.Vars(r)
	from := vars["from"]
	to := vars["to"]

	if stmt, err = pDB.Prepare(CANDUMPSQL); err != nil {
		ReturnJSONError(w, "canDump", err, 500, true)
		return
	}
	now := time.Now()
	year, month, day := now.Date()
	filename := fmt.Sprintf("./log-%d-%02d-%02d-%02d-%02d.trc", year, month, day, now.Hour(), now.Minute())
	file, err := os.Create(filename)
	if err != nil {
		ReturnJSONError(w, "canDump", err, 500, true)
		return
	}

	rows, err := stmt.Query(from, to)
	if err != nil {
		ReturnJSONError(w, "canDump", err, 500, true)
		return
	}
	ReturnCanDumpResult(w, rows, file)
}

func canRecord(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	to := vars["to"]

	toTime, err := time.ParseInLocation("2006-1-2T15:4:5", to, time.Local)
	if err != nil {
		ReturnJSONError(w, "canDump", err, 400, true)
		return
	}
	n := time.Now()
	if !toTime.After(n) {
		errString := fmt.Sprintf("End of recording (%v) is in the past. (%v)", toTime, n)
		ReturnJSONErrorString(w, "canRecord", errString, 400, true)
		return
	}
	if toTime.Sub(time.Now()) > time.Hour*8 {
		ReturnJSONErrorString(w, "canRecord", "You can only ask for up to 8 hours of on demand CAN logging.", 400, true)
		return
	}

	canBus.setOnDemandRecording(toTime)
	toTimeStr := toTime.Format(time.RFC850)
	_, err = fmt.Fprintf(w, `<html>
	<head>
		<title>Can Recording Started</title>
	</head>
	<body>
		<h1>Now recording CAN data until %s<h2><br />
		<a href="/">Back to menu</a>
	</body>
</html>`, toTimeStr)
}

/**
listCANEvents returns a list of the last 50 CAN events
*/
func listCANEvents(w http.ResponseWriter, _ *http.Request) {
	type Event struct {
		Event    string `json:"event"`
		OnDemand bool   `json:"onDemand"`
	}
	var eventList struct {
		Events []Event `json:"event"`
	}
	rows, err := pDB.Query(LISTCANEVENTSSQL)
	if err != nil {
		ReturnJSONError(w, "canDump", err, 500, true)
		return
	}

	if err != nil {
		ReturnJSONError(w, "database", err, 500, true)
		return
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err)
		}
	}()
	for rows.Next() {
		row := new(Event)
		if err := rows.Scan(&row.Event, &row.OnDemand); err != nil {
			log.Print(err)
		} else {
			eventList.Events = append(eventList.Events, *row)
		}
	}
	if JSON, err := json.Marshal(eventList); err != nil {
		ReturnJSONError(w, "canBus", err, 500, true)
		return
	} else {
		if _, err := fmt.Fprintf(w, string(JSON)); err != nil {
			log.Println("Error returning event list - ", err)
		}
	}
}
