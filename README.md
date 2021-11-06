# opencensus-influxdb-exporter

![Build](https://github.com/starvn/opencensus-influxdb-exporter/actions/workflows/go.yml/badge.svg) [![Go Report Card](https://goreportcard.com/badge/github.com/starvn/opencensus-influxdb-exporter)](https://goreportcard.com/report/github.com/starvn/opencensus-influxdb-exporter) [![GoDoc](https://godoc.org/github.com/opencensus-influxdb-exporter/opencensus-influxdb-exporter?status.svg)](https://godoc.org/github.com/starvn/opencensus-influxdb-exporter)

InfluxDB metrics exporter for OpenCensus.io

## Installation

	$ go get -u github.com/starvn/opencensus-influxdb-exporter

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

