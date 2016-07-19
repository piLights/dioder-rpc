package main

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/piLights/dioder"
)

type Configuration struct {
	BindTo            string
	Pins              dioder.Pins
	Debug             bool
	UpdateURL         string
	ConfigurationFile string
	Password          string
	PiBlaster         string
	ServerName        string
	IPv4Only          bool
	IPv6Only          bool
}

var DioderConfiguration Configuration

func (config *Configuration) WriteConfigurationToFile(fileName string) error {

	serializedConfiguration, error := json.Marshal(config)
	if error != nil {
		return error
	}

	error = ioutil.WriteFile(fileName, serializedConfiguration, os.ModePerm)

	return error
}

func NewConfiguration(fileName string) error {
	if fileName == "" {
		DioderConfiguration = Configuration{}
		return nil
	}

	content, error := ioutil.ReadFile(fileName)
	if error != nil {
		return error
	}

	error = json.Unmarshal(content, &DioderConfiguration)

	return error
}