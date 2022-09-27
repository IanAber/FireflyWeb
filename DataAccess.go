package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"time"
)

/**
Get fuel cell recorded values
*/
func getHistory(w http.ResponseWriter, r *http.Request) {
	type SQLRow struct {
		Logged            string
		TankPressure      sql.NullFloat64
		FuelCellPressure  sql.NullFloat64
		WaterConductivity sql.NullFloat64
		ACPower           sql.NullFloat64
		ACCurrent         sql.NullFloat64
		ACVoltage         sql.NullFloat64
		ACFrequency       sql.NullFloat64
		ACPowerFactor     sql.NullFloat64
		ACEnergy          sql.NullFloat64
		HPPower           sql.NullFloat64
		HPCurrent         sql.NullFloat64
		HPVoltage         sql.NullFloat64
		HPFrequency       sql.NullFloat64
		HPPowerFactor     sql.NullFloat64
		HPEnergy          sql.NullFloat64
	}
	type Row struct {
		Logged            string  `json:"logged"`
		TankPressure      float64 `json:"tankPressure"`
		FuelCellPressure  float32 `json:"fcPressure"`
		WaterConductivity float32 `json:"water"`
		ACPower           float64 `json:"acPower"`
		ACCurrent         float64 `json:"acCurrent"`
		ACVoltage         float64 `json:"acVoltage"`
		ACFrequency       float64 `json:"acFrequency"`
		ACPowerFactor     float64 `json:"acPowerFactor"`
		ACEnergy          float64 `json:"acEnergy"`
		HPPower           float64 `json:"hpPower"`
		HPCurrent         float64 `json:"hpCurrent"`
		HPVoltage         float64 `json:"hpVoltage"`
		HPFrequency       float64 `json:"hpFrequency"`
		HPPowerFactor     float64 `json:"hpPowerFactor"`
		HPEnergy          float64 `json:"hpEnergy"`
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
		rows, err = pDB.Query(`SELECT UNIX_TIMESTAMP(logged)
													, gasTankPressure
												 	, gasFuelCellPressure
												 	, totalDissolvedSolids
												 	, ACPower
												 	, ACVolts
												 	, ACFrequency
												 	, ACCurrent
													, ACPowerFactor
     												, ACEnergy
												 	, HPPower
												 	, HPVolts
												 	, HPFrequency
												 	, HPCurrent
													, HPPowerFactor
													, HPEnergy
												FROM firefly.logging
											   WHERE logged BETWEEN ? AND ?`, from, to)
	} else {
		rows, err = pDB.Query(`SELECT (UNIX_TIMESTAMP(logged) DIV 60) * 60
													, ROUND(AVG(gasTankPressure),1)
												 	, ROUND(AVG(gasFuelCellPressure),1)
												 	, ROUND(AVG(totalDissolvedSolids),1)
												 	, ROUND(AVG(ACPower),1)
												 	, ROUND(AVG(ACVolts),1)
												 	, ROUND(AVG(ACFrequency),1)
												 	, ROUND(AVG(ACCurrent),1)
													, ROUND(AVG(ACPowerFactor), 1)
     												, ROUND(AVG(ACEnergy), 1)
												 	, ROUND(AVG(HPPower),1)
												 	, ROUND(AVG(HPVolts),1)
												 	, ROUND(AVG(HPFrequency),1)
												 	, ROUND(AVG(HPCurrent),1)
													, ROUND(AVG(HPPowerFactor), 1)
     												, ROUND(AVG(HPEnergy), 1)
												FROM firefly.logging
											   WHERE logged BETWEEN ? AND ?
											   GROUP BY UNIX_TIMESTAMP(logged) DIV 60`, from, to)
	}

	if err != nil {
		ReturnJSONError(w, "Misc History", err, http.StatusInternalServerError, true)
		return
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Println("Error closing query - ", err)
		}
	}()
	var sqlRow SQLRow
	for rows.Next() {
		if err := rows.Scan(&(sqlRow.Logged), &(sqlRow.TankPressure), &(sqlRow.FuelCellPressure),
			&(sqlRow.WaterConductivity), &(sqlRow.ACPower), &(sqlRow.ACVoltage), &(sqlRow.ACFrequency), &(sqlRow.ACCurrent), &(sqlRow.ACPowerFactor), &(sqlRow.ACEnergy),
			&(sqlRow.HPPower), &(sqlRow.HPVoltage), &(sqlRow.HPFrequency), &(sqlRow.HPCurrent), &(sqlRow.HPPowerFactor), &(sqlRow.HPEnergy)); err != nil {
			log.Print(err)
		} else {
			row := new(Row)
			row.Logged = sqlRow.Logged
			if sqlRow.TankPressure.Valid {
				row.TankPressure = params.ConvertTankPressure(uint16(sqlRow.TankPressure.Float64))
			}
			if sqlRow.FuelCellPressure.Valid {
				row.FuelCellPressure = params.ConvertFuelCellPressure(uint16(sqlRow.FuelCellPressure.Float64))
			}
			if sqlRow.WaterConductivity.Valid {
				row.WaterConductivity = params.ConvertWaterConductivity(uint16(sqlRow.WaterConductivity.Float64))
			}
			if sqlRow.ACPower.Valid {
				row.ACPower = sqlRow.ACPower.Float64 / 100
			}
			if sqlRow.ACVoltage.Valid {
				row.ACVoltage = sqlRow.ACVoltage.Float64 / 100
			}
			if sqlRow.ACFrequency.Valid {
				row.ACFrequency = sqlRow.ACFrequency.Float64 / 100
			}
			if sqlRow.ACCurrent.Valid {
				row.ACCurrent = sqlRow.ACCurrent.Float64 / 100
			}
			if sqlRow.ACPowerFactor.Valid {
				row.ACPowerFactor = sqlRow.ACPowerFactor.Float64 / 100
			}
			if sqlRow.ACEnergy.Valid {
				row.ACEnergy = sqlRow.ACEnergy.Float64
			}
			if sqlRow.HPPower.Valid {
				row.HPPower = sqlRow.HPPower.Float64 / 100
			}
			if sqlRow.HPVoltage.Valid {
				row.HPVoltage = sqlRow.HPVoltage.Float64 / 100
			}
			if sqlRow.HPFrequency.Valid {
				row.HPFrequency = sqlRow.HPFrequency.Float64 / 100
			}
			if sqlRow.HPCurrent.Valid {
				row.HPCurrent = sqlRow.HPCurrent.Float64 / 100
			}
			if sqlRow.HPPowerFactor.Valid {
				row.HPPowerFactor = sqlRow.HPPowerFactor.Float64 / 100
			}
			if sqlRow.HPEnergy.Valid {
				row.HPEnergy = sqlRow.HPEnergy.Float64
			}
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
