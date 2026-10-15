package config

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Settings struct {
	Server struct {
		Listen string `yaml:"listen"`
	} `yaml:"server"`
	Log struct {
		Format string `yaml:"format"`
		Level  string `yaml:"level"`
	} `yaml:"log"`
	FailOn string `yaml:"fail_on"`
}

func LoadSettings(path string) (Settings, error) {
	s := Settings{}
	s.Server.Listen = ":8080"
	s.Log.Format = "json"
	s.Log.Level = "info"
	s.FailOn = "low"
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return s, err
		}
		dec := yaml.NewDecoder(bytes.NewReader(data))
		dec.KnownFields(true)
		if err := dec.Decode(&s); err != nil {
			return s, fmt.Errorf("settings: %w", err)
		}
		var extra any
		if err := dec.Decode(&extra); err != io.EOF {
			return s, fmt.Errorf("multiple settings documents are not supported")
		}
	}
	if v := os.Getenv("WCG_LISTEN"); v != "" {
		s.Server.Listen = v
	}
	if v := os.Getenv("WCG_LOG_FORMAT"); v != "" {
		s.Log.Format = v
	}
	if v := os.Getenv("WCG_LOG_LEVEL"); v != "" {
		s.Log.Level = v
	}
	if v := os.Getenv("WCG_FAIL_ON"); v != "" {
		s.FailOn = v
	}
	return s, s.Validate()
}

func (s Settings) Validate() error {
	if _, _, err := net.SplitHostPort(s.Server.Listen); err != nil {
		return fmt.Errorf("server.listen: %w", err)
	}
	if s.Log.Format != "json" && s.Log.Format != "text" {
		return fmt.Errorf("log.format must be json or text")
	}
	if !strings.Contains(" debug info warn error ", " "+s.Log.Level+" ") {
		return fmt.Errorf("invalid log.level")
	}
	if !strings.Contains(" low medium high ", " "+s.FailOn+" ") {
		return fmt.Errorf("fail_on must be low, medium or high")
	}
	return nil
}
