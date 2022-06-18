package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
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
	ElectrolyserHoldOffTime   time.Duration `json:"electrolyserHoldOffTime"`
	ElectrolyserHoldOnTime    time.Duration `json:"electrolyserHoldOnTime"`
	ElectrolyserOffDelay      time.Duration `json:"electrolyserOffDelay"`
	ElectrolyserShutDownDelay time.Duration `json:"electrolyserShutDownDelay"`
}

func NewJsonSettings() *JsonSettings {
	s := new(JsonSettings)
	s.ElectrolyserHoldOffTime = ELECTROLYSERHOLDOFFTIME
	s.ElectrolyserHoldOnTime = ELECTROLYSERHOLDONTIME
	s.ElectrolyserOffDelay = ELECTROLYSEROFFDELAYTIME
	s.ElectrolyserShutDownDelay = ELECTROLYSERSHUTDOWNDELAY
	return s
}

func (s *JsonSettings) ReadSettings(filepath string) error {
	if file, err := ioutil.ReadFile(filepath); err != nil {
		return err
	} else {
		if err := json.Unmarshal(file, s); err != nil {
			return err
		}
	}
	return nil
}

func (s *JsonSettings) WriteSettings(filepath string) error {
	if bData, err := json.Marshal(s); err != nil {
		log.Println("Error converting settings to text -", err)
		return err
	} else {
		if err = ioutil.WriteFile(filepath, bData, 0644); err != nil {
			log.Println("Error writing JSON settings file -", err)
			return err
		}
	}
	return nil
}
