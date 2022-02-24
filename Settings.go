package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
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
