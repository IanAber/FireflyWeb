package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type errorObj struct {
	Device string
	Err    string
}

type JSONError struct {
	Errors []*errorObj
}

func (j *JSONError) AddErrorString(device string, err string) {
	e := new(errorObj)
	e.Device = device
	e.Err = err
	j.Errors = append(j.Errors, e)
}

func (j *JSONError) AddError(device string, err error) {
	e := new(errorObj)
	e.Device = device
	e.Err = err.Error()
	j.Errors = append(j.Errors, e)
}

func (j *JSONError) String() string {
	if s, err := json.Marshal(j); err != nil {
		log.Print(err)
		return ""
	} else {
		return string(s)
	}
}

func (j *JSONError) ReturnError(w http.ResponseWriter, retCode int) {
	w.WriteHeader(retCode)
	_, err := fmt.Print(w, j.String())
	if err != nil {
		log.Println(err)
	}
}