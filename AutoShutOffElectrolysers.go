package main

import (
	"log"
	"time"
)

/***************
Manages the daily shutting down of the electrolysers.
*/

func CalculateOffTime() {
	var lastProductionTime time.Time
	if pDB == nil {
		if dbPtr, err := connectToDatabase(); err != nil {
			log.Print(err)
			return
		} else {
			pDB = dbPtr
		}
	}
	if rows, err := pDB.Query("select concat(current_date, \"T\", max(time(logged))) as logged from logging where el1StateCode = 3 and logged > date_add(current_date, interval -7 day)"); err != nil {
		log.Print(err)
		return
	} else {
		for rows.Next() {
			s := ""
			if err := rows.Scan(&s); err != nil {
				log.Print(err)
				return
			} else {
				lastProductionTime, err = time.ParseInLocation("2006-01-02T15:04:05", s, time.Local)
				if err != nil {
					log.Print(err)
				}
				electrolyserShutDownTime = lastProductionTime.Add(params.ElectrolyserShutDownDelay)
			}
		}
	}
	_, err := pDB.Exec("call firefly.archive_logging")
	if err != nil {
		log.Println("Archiver failed - ", err)
	}
}

func ShutDownElectrolysers() bool {
	if !SystemStatus.Relays.Electrolyser1 && !SystemStatus.Relays.Electrolyser2 {
		// Already off
		return true
	}
	log.Println("Auto-shutting down electrolysers.")
	for _, el := range SystemStatus.Electrolysers {
		if el.status.StackVoltage > 30 {
			// Stack voltage on one electrolyser is too high
			return false
		}
	}
	strCommand := "el2 off"
	if _, err := sendCommand(strCommand); err != nil {
		log.Print(err)
		return false
	}

	strCommand = "el1dr off"
	if _, err := sendCommand(strCommand); err != nil {
		log.Print(err)
		return false
	}
	return true
}
