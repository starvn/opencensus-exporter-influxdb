# opencensus-exporter-influxdb

![Build](https://github.com/starvn/opencensus-exporter-influxdb/actions/workflows/go.yml/badge.svg) [![Go Report Card](https://goreportcard.com/badge/github.com/starvn/opencensus-exporter-influxdb)](https://goreportcard.com/report/github.com/starvn/opencensus-exporter-influxdb) [![GoDoc](https://godoc.org/github.com/opencensus-exporter-influxdb/opencensus-exporter-influxdb?status.svg)](https://godoc.org/github.com/starvn/opencensus-exporter-influxdb)

InfluxDB metrics exporter for OpenCensus.io

## Installation

	$ go get -u github.com/starvn/opencensus-exporter-influxdb

## Register

	e, err := influxdb.NewExporter(influxdb.Options{
		Database: "db",
		Address:  "http://example.tld",
		Username: "user",
		Password: "password",
	})
	if err != nil {
		log.Fatalf("unexpected error: %s", err.Error())
	}
	view.RegisterExporter(e)

