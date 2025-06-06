package main

// Configuration structures
type Probe struct {
	Name    string `json:"name"`
	Src     string `json:"src"`
	Dst     string `json:"dst"`
	Size    int    `json:"size"`
	Timeout int    `json:"timeout"`
}

type ProbeConfig struct {
	Interval int `json:"interval"`
}

type ReportingConfig struct {
	Interval int `json:"interval"`
}

type CloudWatchConfig struct {
	Region    string `json:"region"`
	Namespace string `json:"namespace"`
	Interval  int    `json:"interval"`
}

type GlobalConfig struct {
	Reporting  ReportingConfig  `json:"reporting"`
	Probe      ProbeConfig      `json:"probe"`
	CloudWatch CloudWatchConfig `json:"cloudwatch"`
}

type Config struct {
	Conf   GlobalConfig `json:"conf"`
	Probes []Probe      `json:"probes"`
}
