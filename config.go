package main

type FeoConfig struct {
	Controller string `yaml:"controller"`
	Scheme     string `yaml:"scheme"`
	Host       string `yaml:"host"`
	Policy     struct {
		Name   string `yaml:"name"`
		Config string `yaml:"config"`
	} `yaml:"policy"`
	Peers []string `yaml:"peers"`
}
