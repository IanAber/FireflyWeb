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
	if rows, err := pDB.Query(`select ifnull(concat(current_date, "T", greatest(max(time(logged)), time('18:00'))),concat(current_date, "T18:00:00")) as logged
from logging
where el1StateCode = 3 and logged > date_add(current_date, interval -7 day)`); err != nil {
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
	if !SystemStatus.Relays.EL0 && !SystemStatus.Relays.EL1 {
		// Already off
		return true
	}
	for _, el := range SystemStatus.Electrolysers {
		if el.status.StackVoltage > 30 {
			// Stack voltage on one electrolyser is too high
			return false
		}
	}
	log.Println("Auto-shutting down electrolysers.")
	if err := mbusRTU.EL1OnOff(false); err != nil {
		log.Print(err)
		return false
	}

	if err := mbusRTU.EL0OnOff(false); err != nil {
		log.Print(err)
		return false
	}
	return true
}
