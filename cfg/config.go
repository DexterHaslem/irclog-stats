package cfg

import (
	"encoding/json"
	"io/ioutil"
)

// C is the configuration options for irc logger settings, mainly database
// credentials
type C struct {
	DBName   string `json:"db"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// From will try to load a config object from a file in json format
func From(file string) (*C, error) {
	fc, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	ret := &C{}
	err = json.Unmarshal(fc, ret)
	return ret, err
}

// Save will save the current configuration object to a given filename in
// json format
func (c *C) Save(file string) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, b, 0666)
}
