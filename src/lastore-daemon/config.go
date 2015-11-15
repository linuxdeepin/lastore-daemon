package main

type Config struct {
	AutoCheckUpdate bool
	MirrorServer    string
	CheckInterval   int

	fpath string
}

func NewConfig(fpath string) *Config {
	return &Config{
		fpath:           fpath,
		CheckInterval:   60 * 10,
		MirrorServer:    "default",
		AutoCheckUpdate: true,
	}
}

func (*Config) SetAutoCheckUpdate() {
}

func (*Config) SetMirrorServer(id string) {
}
