package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Settings struct {
	values map[string]string
}

func (s *Settings) ReadSettings(filepath string) error {
	s.values = make(map[string]string)
	file, err := ioutil.ReadFile(filepath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(file), "\n")
	for _, line := range lines {
		if len(line) > 0 && line[0] != '#' {
			parts := strings.Split(line, "=")
			if len(parts) < 2 {
				parts = strings.Split(line, " ")
			}
			if len(parts) < 2 {
				log.Println("Can't split key and value in ", line)
			} else {
				s.values[strings.Trim(parts[0], " \t")] = strings.Trim(parts[1], " \t")
			}
		}
	}
	return nil
}

func (s *Settings) GetInt8Setting(key string) (uint8, error) {
	val, exists := s.values[key]
	if !exists {
		return 0, fmt.Errorf("%s not found in the settings file.", key)
	}
	iVal, err := strconv.ParseInt(val, 10, 8)
	if err != nil {
		return 0, err
	}
	return uint8(iVal), nil
}

func (s *Settings) GetList(key string) ([]string, error) {
	val, exists := s.values[key]
	if !exists {
		return nil, fmt.Errorf("%s not found in the settings file.", key)
	}
	return strings.Split(val, ",;"), nil
}

type JsonSettings struct {
	ElectrolyserHoldOffTime          time.Duration `json:"electrolyserHoldOffTime"`
	ElectrolyserHoldOnTime           time.Duration `json:"electrolyserHoldOnTime"`
	ElectrolyserOffDelay             time.Duration `json:"electrolyserOffDelay"`
	ElectrolyserShutDownDelay        time.Duration `json:"electrolyserShutDownDelay"`
	ElectrolyserMaxStackVoltsTurnOff int           `json:"electrolyserMaxStackVoltsForShutdown"`
	FuelCellMaintenance              bool          `json:"fuelCellMaintenance"`
	FuelCellMaxRestarts              int           `json:"fuelCellMaxRestarts"`
	FuelCellRestartOffTime           time.Duration `json:"fuelCellRestartOffTime"`
	FuelCellEnableToRunDelay         time.Duration `json:"fuelCellEnableToRunDelay"`
	FuelCellLogOnRun                 bool          `json:"fuelCellLogOnRun"`
	FuelCellLogOnEnable              bool          `json:"fuelCellLogOnEnable"`
	GasOnDelay                       time.Duration `json:"gasOnDelay"`
	GasOffDelay                      time.Duration `json:"gasOffDelay"`
	DebugOutput                      bool          `json:"debugOutputEnable"`
	filepath                         string
}

func NewJsonSettings() *JsonSettings {
	s := new(JsonSettings)
	s.ElectrolyserHoldOffTime = ELECTROLYSERHOLDOFFTIME
	s.ElectrolyserHoldOnTime = ELECTROLYSERHOLDONTIME
	s.ElectrolyserOffDelay = ELECTROLYSEROFFDELAYTIME
	s.ElectrolyserShutDownDelay = ELECTROLYSERSHUTDOWNDELAY
	s.ElectrolyserMaxStackVoltsTurnOff = ELECTROLYSERMAXSTACKVOLTSFORTURNOFF
	s.FuelCellMaintenance = true
	s.FuelCellMaxRestarts = MAXFUELCELLRESTARTS
	s.FuelCellRestartOffTime = OFFTIMEFORFUELCELLRESTART
	s.FuelCellEnableToRunDelay = FUELCELLENABLETORUNDELAY
	s.GasOnDelay = GASONDELAY
	s.GasOffDelay = GASOFFDELAY
	s.FuelCellLogOnRun = false
	s.FuelCellLogOnEnable = false
	s.GasOnDelay = GASONDELAY
	s.DebugOutput = true
	return s
}

func (s *JsonSettings) ReadSettings(filepath string) error {
	s.filepath = filepath
	if file, err := ioutil.ReadFile(filepath); err != nil {
		return err
	} else {
		if err := json.Unmarshal(file, s); err != nil {
			return err
		}
	}
	return nil
}

func (s *JsonSettings) WriteSettings() error {
	if bData, err := json.Marshal(s); err != nil {
		log.Println("Error converting settings to text -", err)
		return err
	} else {
		if err = ioutil.WriteFile(s.filepath, bData, 0644); err != nil {
			log.Println("Error writing JSON settings file -", err)
			return err
		}
	}
	return nil
}

/***
printMinutesOptions generates a set of options for a select list for picking a number for a delay time
*/
func printOptions(w http.ResponseWriter, setting int, min int, max int, units string, variableName string, labelText string) {
	var selected string

	if _, err := fmt.Fprintf(w, `<label for="%s">%s</label><select id="%s" name="%s">`, variableName, labelText, variableName, variableName); err != nil {
		log.Println(err)
		return
	}
	for m := min; m <= max; m++ {
		if m == setting {
			selected = "selected"
		} else {
			selected = ""
		}
		_, err := fmt.Fprintf(w, "<option value=%d %s>%d %s</option>", m, selected, m, units)
		if err != nil {
			log.Println(err)
			return
		}
	}
	if _, err := fmt.Fprintf(w, `</select><br />`); err != nil {
		log.Println(err)
	}
	return
}

/***
printSwitch generates a switch to select a boolean value
*/
func printSwitch(w http.ResponseWriter, val bool, variableName string, labelText string) {
	checked := ""
	if val {
		checked = "checked"
	}
	if _, err := fmt.Fprintf(w, `<label class="toggle-switchy" for="%s" data-size="" data-style="rounded"><input %s type="checkbox" id="%s" name="%s"><span class="toggle"><span class="switch"></span></span><span class="label">%s</span></label><br />`,
		variableName, checked, variableName, variableName, labelText); err != nil {
		log.Println(err)
	}
}

func getSettings(w http.ResponseWriter, _ *http.Request) {

	if _, err := fmt.Fprint(w, `<html>
<head>
	<title>FireflyWeb Settings</title>
    <link href="switch_styles.css" rel="stylesheet"></head>
	<style>
              .egButton {
                  -moz-box-shadow:inset 5px 5px 0px -2px #a6827e;
                  -webkit-box-shadow:inset 5px 5px 0px -2px #a6827e;
                  box-shadow:inset 5px 5px 0px -2px #a6827e;
                  background:-webkit-gradient(linear, left top, left bottom, color-stop(0.05, #7d5d3b), color-stop(1, #634b30));
                  background:-moz-linear-gradient(top, #7d5d3b 5%, #634b30 100%);
                  background:-webkit-linear-gradient(top, #7d5d3b 5%, #634b30 100%);
                  background:-o-linear-gradient(top, #7d5d3b 5%, #634b30 100%);
                  background:-ms-linear-gradient(top, #7d5d3b 5%, #634b30 100%);
                  background:linear-gradient(to bottom, #7d5d3b 5%, #634b30 100%);
                  filter:progid:DXImageTransform.Microsoft.gradient(startColorstr='#7d5d3b', endColorstr='#634b30',GradientType=0);
                  background-color:#7d5d3b;
                  -webkit-border-radius:12px;
                  -moz-border-radius:12px;
                  border-radius:12px;
                  border:1px solid #54381e;
                  display:inline-block;
                  cursor:pointer;
                  color:#ffffff;
                  font-family:Arial;
                  font-size:16px;
                  padding:12px 26px;
                  text-decoration:none;
              }
              .egButton:hover {
                  background:-webkit-gradient(linear, left top, left bottom, color-stop(0.05, #634b30), color-stop(1, #7d5d3b));
                  background:-moz-linear-gradient(top, #634b30 5%, #7d5d3b 100%);
                  background:-webkit-linear-gradient(top, #634b30 5%, #7d5d3b 100%);
                  background:-o-linear-gradient(top, #634b30 5%, #7d5d3b 100%);
                  background:-ms-linear-gradient(top, #634b30 5%, #7d5d3b 100%);
                  background:linear-gradient(to bottom, #634b30 5%, #7d5d3b 100%);
                  filter:progid:DXImageTransform.Microsoft.gradient(startColorstr='#634b30', endColorstr='#7d5d3b',GradientType=0);
                  background-color:#634b30;
              }
              .egButton:active {
                  position:relative;
                  top:1px;
              }
	</style>
</head>
<body>
	<form action="./settings" method="POST">`); err != nil {
		log.Println(err)
		return
	}
	printOptions(w, int(params.ElectrolyserHoldOffTime.Minutes()), 1, 30, "minutes", "elholdoff", "Electrolyser hold off time")
	printOptions(w, int(params.ElectrolyserHoldOnTime.Minutes()), 1, 30, "minutes", "elholdon", "Electrolyser hold on time")
	printOptions(w, int(params.ElectrolyserOffDelay.Minutes()), 1, 30, "minutes", "eldelayoff", "Electrolyser off delay time")
	printOptions(w, int(params.ElectrolyserShutDownDelay.Minutes()), 1, 30, "minutes", "elshutdowndelay", "Electrolyser shut down delay time")
	printOptions(w, int(params.ElectrolyserMaxStackVoltsTurnOff), 25, 45, "Volts", "electrolyserMaxStackVoltsForShutdown", "Maximum stack voltage for electrolyser to be turned off")
	printOptions(w, int(params.FuelCellMaxRestarts), 1, 25, "", "fcmaxrestarts", "Fuel Cell Maximumn Resatrts")
	printOptions(w, int(params.FuelCellRestartOffTime.Seconds()), 0, 120, "seconds", "fcrestarttime", "Fuel Cell off time when restarting")
	printOptions(w, int(params.FuelCellEnableToRunDelay.Seconds()), 0, 30, "seconds", "fcenabletorun", "Fuel Cell delay between on and run")
	printOptions(w, int(params.GasOnDelay.Seconds()), 0, 120, "seconds", "gasondelay", "Delay after turning gas on before run")
	printOptions(w, int(params.GasOffDelay.Seconds()), 0, 5, "seconds", "gasoffdelay", "Delay after run before turning gas off")
	printSwitch(w, params.DebugOutput, "debug", "Enable debug output")
	printSwitch(w, params.FuelCellLogOnRun, "logonrun", "Generate fuel cell log when running")
	printSwitch(w, params.FuelCellLogOnEnable, "logonenable", "Generate fuel cell log when enabled")
	printSwitch(w, params.FuelCellMaintenance, "fcmaintenance", "Set fuel cell to maintenance mode")
	if _, err := fmt.Fprint(w, `<br /><button class="egButton" type="submit" >Update Settings</button></form><a href="/">Main Menu</a></body></html>`); err != nil {
		log.Println(err)
	}
}

/**
updateSettings handles the form coming back form the operator with new settings values
*/
func updateSettings(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println(err)
		if _, perr := fmt.Fprintf(w, `<html><head><title>Error</title></head><body>%v</body></html>`, err); perr != nil {
			log.Println(perr)
		}
		return
	}
	holdoffTime := r.Form.Get("elholdoff")
	holdonTime := r.Form.Get("elholdon")
	delayOff := r.Form.Get("eldelayoff")
	delayShutDown := r.Form.Get("elshutdowndelay")
	fcMaxRestarts := r.Form.Get("fcmaxrestarts")
	fcRestartOffTime := r.Form.Get("fcrestarttime")
	fcEnableToRunTime := r.Form.Get("fcenabletorun")
	gasOnDelay := r.Form.Get("gasondelay")
	gasOffDelay := r.Form.Get("gasoffdelay")
	debug := r.Form.Get("debug")
	logOnRun := r.Form.Get("logonrun")
	logOnEnable := r.Form.Get("logonenable")
	maintenance := r.Form.Get("fcmaintenance")

	if len(holdoffTime) > 0 {
		t, err := strconv.Atoi(holdoffTime)
		if err != nil {
			log.Println(err)
		} else {
			params.ElectrolyserHoldOffTime = time.Minute * time.Duration(t)
		}
	}
	if len(holdonTime) > 0 {
		t, err := strconv.Atoi(holdonTime)
		if err != nil {
			log.Println(err)
		} else {
			params.ElectrolyserHoldOnTime = time.Minute * time.Duration(t)
		}
	}
	if len(delayOff) > 0 {
		t, err := strconv.Atoi(delayOff)
		if err != nil {
			log.Println(err)
		} else {
			params.ElectrolyserOffDelay = time.Minute * time.Duration(t)
		}
	}
	if len(delayShutDown) > 0 {
		t, err := strconv.Atoi(delayShutDown)
		if err != nil {
			log.Println(err)
		} else {
			params.ElectrolyserShutDownDelay = time.Minute * time.Duration(t)
		}
	}
	if len(fcMaxRestarts) > 0 {
		t, err := strconv.Atoi(fcMaxRestarts)
		if err != nil {
			log.Println(err)
		} else {
			params.FuelCellMaxRestarts = t
		}
	}
	if len(fcRestartOffTime) > 0 {
		t, err := strconv.Atoi(fcRestartOffTime)
		if err != nil {
			log.Println(err)
		} else {
			params.FuelCellRestartOffTime = time.Second * time.Duration(t)
		}
	}
	if len(fcEnableToRunTime) > 0 {
		t, err := strconv.Atoi(fcEnableToRunTime)
		if err != nil {
			log.Println(err)
		} else {
			params.FuelCellEnableToRunDelay = time.Second * time.Duration(t)
		}
	}
	if len(gasOnDelay) > 0 {
		t, err := strconv.Atoi(gasOnDelay)
		if err != nil {
			log.Println(err)
		} else {
			params.GasOnDelay = time.Second * time.Duration(t)
		}
	}
	if len(gasOffDelay) > 0 {
		t, err := strconv.Atoi(gasOffDelay)
		if err != nil {
			log.Println(err)
		} else {
			params.GasOffDelay = time.Second * time.Duration(t)
		}
	}
	params.DebugOutput = (len(debug) > 0)
	if !params.FuelCellLogOnEnable && (len(logOnEnable) > 0) {
		// We are enabling a log on eneable here so we should set the event date/time
		canBus.setEventDateTime()
	}
	params.FuelCellLogOnEnable = (len(logOnEnable) > 0)

	// Log On Enable overrides Log On Run if it is set
	params.FuelCellLogOnRun = false
	if !params.FuelCellLogOnEnable {
		if !params.FuelCellLogOnRun && (len(logOnRun) > 0) {
			// We are starting a log on run so set the event date/time
			canBus.setEventDateTime()
		}
		params.FuelCellLogOnRun = (len(logOnRun) > 0)
	}
	params.FuelCellMaintenance = (len(maintenance) > 0)

	if err := params.WriteSettings(); err != nil {
		log.Println(err)
	}
	getSettings(w, nil)
}
